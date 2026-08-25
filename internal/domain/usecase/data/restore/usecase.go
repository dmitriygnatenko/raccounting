// Package restore is the RestoreData use case: it replaces every table raccounting stores with the
// contents of a previously exported entity.Backup (see the export package). raccounting is
// single-user, so this is a full restore, not a merge — anything currently stored is deleted first.
//
// Account balances aren't copied from the backup directly. Instead, each account is recreated with
// its balance as it was *before* any of the backup's transactions, and every transaction/transfer is
// then replayed on top, oldest first — the same sequence of Create/CreateTransfer calls (and the
// same balance-never-negative checks) that originally built up that balance. This is what lets
// restore reuse the ordinary repositories instead of writing balances directly: by construction, if
// the backup is internally consistent, replay reaches exactly the balance it was exported with.
package restore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	"raccounting/internal/domain/entity"
	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements RestoreData.
type UseCase struct {
	userRepository           port.UserRepository
	accountRepository        port.AccountRepository
	categoryRepository       port.CategoryRepository
	currencyRepository       port.CurrencyRepository
	tagRepository            port.TagRepository
	transactionRepository    port.TransactionRepository
	categoryBudgetRepository port.BudgetRepository
}

// New builds a UseCase from its dependencies.
func New(
	userRepository port.UserRepository,
	accountRepository port.AccountRepository,
	categoryRepository port.CategoryRepository,
	currencyRepository port.CurrencyRepository,
	tagRepository port.TagRepository,
	transactionRepository port.TransactionRepository,
	categoryBudgetRepository port.BudgetRepository,
) *UseCase {
	return &UseCase{
		userRepository:           userRepository,
		accountRepository:        accountRepository,
		categoryRepository:       categoryRepository,
		currencyRepository:       currencyRepository,
		tagRepository:            tagRepository,
		transactionRepository:    transactionRepository,
		categoryBudgetRepository: categoryBudgetRepository,
	}
}

// Execute validates input.Backup, then wipes and recreates every table from it. There is no
// undo — see the package doc for why Validate must reject anything wipe/recreate can't safely
// finish restoring.
func (uc *UseCase) Execute(ctx context.Context, input Input) error {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "restore data: validation", "error", err)

		return domainError.ToValidationError(err)
	}

	if err := uc.wipe(ctx); err != nil {
		slog.ErrorContext(ctx, "restore data: wipe", "error", err)

		return errors.New("Failed to clear existing data")
	}

	if err := uc.recreate(ctx, input); err != nil {
		slog.ErrorContext(ctx, "restore data: recreate", "error", err)

		return errors.New("Failed to restore data — existing data has already been cleared; try importing again")
	}

	return nil
}

// wipe deletes every existing row across every table restore touches, in the only order that's safe
// given the FK/balance constraints each repository enforces: transactions first (newest first, so
// each delete reverses exactly the balance effect its own create applied, undo-stack style — deleting
// in any other order can spuriously fail accounts.balance's CHECK (balance >= 0)), then budgets, tags
// and categories, then accounts, then currencies.
func (uc *UseCase) wipe(ctx context.Context) error {
	transactions, err := uc.transactionRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("list transactions: %w", err)
	}

	sort.Slice(transactions, func(i, j int) bool { return transactions[i].ID > transactions[j].ID })

	done := make(map[uint64]bool, len(transactions))

	for _, tx := range transactions {
		if done[tx.ID] {
			continue
		}

		done[tx.ID] = true

		if tx.IsTransfer() {
			if tx.TransferTransactionID != nil {
				done[*tx.TransferTransactionID] = true
			}

			if _, err := uc.transactionRepository.DeleteTransfer(ctx, tx.ID); err != nil {
				return fmt.Errorf("delete transfer %d: %w", tx.ID, err)
			}

			continue
		}

		if err := uc.transactionRepository.Delete(ctx, tx.ID); err != nil {
			return fmt.Errorf("delete transaction %d: %w", tx.ID, err)
		}
	}

	budgets, err := uc.categoryBudgetRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("list budgets: %w", err)
	}

	for _, b := range budgets {
		if err := uc.categoryBudgetRepository.Set(ctx, port.BudgetSetRequest{
			CategoryID: b.CategoryID,
			MonthKey:   b.MonthKey,
			Amount:     0,
		}); err != nil {
			return fmt.Errorf("clear budget: %w", err)
		}
	}

	tags, err := uc.tagRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("list tags: %w", err)
	}

	for _, t := range tags {
		if err := uc.tagRepository.Delete(ctx, t.ID); err != nil {
			return fmt.Errorf("delete tag %d: %w", t.ID, err)
		}
	}

	categories, err := uc.categoryRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("list categories: %w", err)
	}

	for _, c := range categories {
		if err := uc.categoryRepository.Delete(ctx, c.ID); err != nil {
			return fmt.Errorf("delete category %d: %w", c.ID, err)
		}
	}

	accounts, err := uc.accountRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("list accounts: %w", err)
	}

	for _, a := range accounts {
		if err := uc.accountRepository.Delete(ctx, a.ID); err != nil {
			return fmt.Errorf("delete account %d: %w", a.ID, err)
		}
	}

	currencies, err := uc.currencyRepository.List(ctx)
	if err != nil {
		return fmt.Errorf("list currencies: %w", err)
	}

	for _, c := range currencies {
		if err := uc.currencyRepository.Delete(ctx, c.Code); err != nil {
			return fmt.Errorf("delete currency %s: %w", c.Code, err)
		}
	}

	return nil
}

// recreate rebuilds every table from input.Backup, in dependency order: currencies, then accounts
// (whose opening balance is derived — see the package doc), then categories and tags, then every
// transaction/transfer replayed oldest-first, then budgets, then the signed-in user's language.
func (uc *UseCase) recreate(ctx context.Context, input Input) error {
	backup := input.Backup

	if err := uc.recreateCurrencies(ctx, backup.Currencies); err != nil {
		return err
	}

	accountIDs, accountCurrencies, err := uc.recreateAccounts(ctx, backup.Accounts, backup.Transactions)
	if err != nil {
		return err
	}

	categoryIDs, err := uc.recreateCategories(ctx, backup.Categories)
	if err != nil {
		return err
	}

	tagIDs, err := uc.recreateTags(ctx, backup.Tags)
	if err != nil {
		return err
	}

	if err := uc.replayTransactions(
		ctx, backup.Transactions, accountIDs, accountCurrencies, categoryIDs, tagIDs,
	); err != nil {
		return err
	}

	for _, b := range backup.Budgets {
		if b.Amount <= 0 {
			continue
		}

		categoryID, ok := categoryIDs[b.CategoryID]
		if !ok {
			continue
		}

		if err := uc.categoryBudgetRepository.Set(ctx, port.BudgetSetRequest{
			CategoryID: categoryID,
			MonthKey:   b.MonthKey,
			Amount:     b.Amount,
		}); err != nil {
			return fmt.Errorf("set budget: %w", err)
		}
	}

	if backup.Settings.Language != "" || backup.Settings.Theme != "" {
		if err := uc.userRepository.UpdateSettings(ctx, input.UserID, backup.Settings); err != nil {
			return fmt.Errorf("update settings: %w", err)
		}
	}

	return nil
}

func (uc *UseCase) recreateCurrencies(ctx context.Context, currencies []entity.Currency) error {
	for _, cur := range currencies {
		created, err := uc.currencyRepository.Create(ctx, port.CurrencyCreateRequest{
			Code:    cur.Code,
			Symbol:  cur.Symbol,
			Name:    cur.Name,
			Rate:    cur.Rate,
			Default: cur.Default,
		})
		if err != nil {
			return fmt.Errorf("create currency %s: %w", cur.Code, err)
		}

		if cur.Status != entity.CurrencyStatusArchived {
			continue
		}

		if _, err := uc.currencyRepository.Update(ctx, port.CurrencyUpdateRequest{
			Code:     created.Code,
			Symbol:   created.Symbol,
			Name:     created.Name,
			Rate:     created.Rate,
			Default:  created.Default,
			Archived: true,
		}); err != nil {
			return fmt.Errorf("archive currency %s: %w", cur.Code, err)
		}
	}

	return nil
}

// recreateAccounts creates every account with its opening balance — the exported (current) balance
// minus the net effect of every transaction the backup will replay onto it — so replaying those
// transactions on top lands back on the exact exported balance. Returns the old id -> new id map and
// the new id -> currency code map replayTransactions needs.
func (uc *UseCase) recreateAccounts(
	ctx context.Context, accounts []entity.Account, transactions []entity.Transaction,
) (ids map[uint64]uint64, currencies map[uint64]string, err error) {
	netEffect := make(map[uint64]int64, len(accounts))
	for _, tx := range transactions {
		netEffect[tx.AccountID] += tx.Amount
	}

	ids = make(map[uint64]uint64, len(accounts))
	currencies = make(map[uint64]string, len(accounts))

	for _, acc := range accounts {
		opening := acc.Balance - netEffect[acc.ID]
		if opening < 0 {
			return nil, nil, fmt.Errorf(
				"account %q: reconstructed opening balance is negative — the backup's transactions "+
					"don't add up to its exported balance", acc.Name,
			)
		}

		created, err := uc.accountRepository.Create(ctx, port.AccountCreateRequest{
			Name:         acc.Name,
			Type:         acc.Type,
			CurrencyCode: acc.CurrencyCode,
			Balance:      opening,
		})
		if err != nil {
			return nil, nil, fmt.Errorf("create account %q: %w", acc.Name, err)
		}

		ids[acc.ID] = created.ID
		currencies[created.ID] = created.CurrencyCode

		if acc.Status != entity.AccountStatusArchived {
			continue
		}

		if _, err := uc.accountRepository.Update(ctx, port.AccountUpdateRequest{
			ID:           created.ID,
			Name:         created.Name,
			Type:         created.Type,
			CurrencyCode: created.CurrencyCode,
			Archived:     true,
		}); err != nil {
			return nil, nil, fmt.Errorf("archive account %q: %w", acc.Name, err)
		}
	}

	return ids, currencies, nil
}

func (uc *UseCase) recreateCategories(ctx context.Context, categories []entity.Category) (map[uint64]uint64, error) {
	ids := make(map[uint64]uint64, len(categories))

	for _, cat := range categories {
		created, err := uc.categoryRepository.Create(ctx, port.CategoryCreateRequest{
			Name:  cat.Name,
			Type:  cat.Type,
			Color: cat.Color,
		})
		if err != nil {
			return nil, fmt.Errorf("create category %q: %w", cat.Name, err)
		}

		ids[cat.ID] = created.ID

		if cat.Status != entity.CategoryStatusArchived {
			continue
		}

		if _, err := uc.categoryRepository.Update(ctx, port.CategoryUpdateRequest{
			ID:       created.ID,
			Name:     created.Name,
			Color:    created.Color,
			Archived: true,
		}); err != nil {
			return nil, fmt.Errorf("archive category %q: %w", cat.Name, err)
		}
	}

	return ids, nil
}

func (uc *UseCase) recreateTags(ctx context.Context, tags []entity.Tag) (map[uint64]uint64, error) {
	ids := make(map[uint64]uint64, len(tags))

	for _, tag := range tags {
		created, err := uc.tagRepository.Create(ctx, port.TagCreateRequest{
			Name:  tag.Name,
			Color: tag.Color,
		})
		if err != nil {
			return nil, fmt.Errorf("create tag %q: %w", tag.Name, err)
		}

		ids[tag.ID] = created.ID
	}

	return ids, nil
}

// replayTransactions recreates every transaction/transfer oldest-first (by its id in the backup,
// which reflects the order it was originally created in) — see the package doc for why the order
// matters. Input.Validate already checked every reference this dereferences.
func (uc *UseCase) replayTransactions(
	ctx context.Context,
	transactions []entity.Transaction,
	accountIDs map[uint64]uint64,
	accountCurrencies map[uint64]string,
	categoryIDs map[uint64]uint64,
	tagIDs map[uint64]uint64,
) error {
	byID := make(map[uint64]entity.Transaction, len(transactions))
	for _, tx := range transactions {
		byID[tx.ID] = tx
	}

	ordered := make([]entity.Transaction, len(transactions))
	copy(ordered, transactions)
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].ID < ordered[j].ID })

	done := make(map[uint64]bool, len(ordered))

	for _, tx := range ordered {
		if done[tx.ID] {
			continue
		}

		done[tx.ID] = true

		if tx.IsTransfer() {
			partner := byID[*tx.TransferTransactionID]
			done[partner.ID] = true

			if err := uc.replayTransfer(ctx, tx, partner, accountIDs, accountCurrencies); err != nil {
				return err
			}

			continue
		}

		var categoryID *uint64
		if tx.CategoryID != nil {
			mapped := categoryIDs[*tx.CategoryID]
			categoryID = &mapped
		}

		tags := make([]uint64, 0, len(tx.TagIDs))
		for _, id := range tx.TagIDs {
			tags = append(tags, tagIDs[id])
		}

		accountID := accountIDs[tx.AccountID]

		if _, err := uc.transactionRepository.Create(ctx, port.TransactionCreateRequest{
			AccountID:    accountID,
			CategoryID:   categoryID,
			Type:         tx.Type,
			CurrencyCode: accountCurrencies[accountID],
			Amount:       tx.Amount,
			Memo:         tx.Memo,
			OperationAt:  tx.OperationAt,
			TagIDs:       tags,
		}); err != nil {
			return fmt.Errorf("create transaction %d: %w", tx.ID, err)
		}
	}

	return nil
}

// replayTransfer recreates one transfer's two legs from tx and its partner leg — whichever of the
// two has the negative (debit) amount is the "from" side.
func (uc *UseCase) replayTransfer(
	ctx context.Context,
	tx, partner entity.Transaction,
	accountIDs map[uint64]uint64,
	accountCurrencies map[uint64]string,
) error {
	from, to := tx, partner
	if from.Amount > 0 {
		from, to = to, from
	}

	rate := 1.0
	if from.TransferRate != nil {
		rate = *from.TransferRate
	}

	fromAccountID := accountIDs[from.AccountID]
	toAccountID := accountIDs[to.AccountID]

	if _, err := uc.transactionRepository.CreateTransfer(ctx, port.TransferCreateRequest{
		FromAccountID:    fromAccountID,
		FromCurrencyCode: accountCurrencies[fromAccountID],
		ToAccountID:      toAccountID,
		ToCurrencyCode:   accountCurrencies[toAccountID],
		Amount:           -from.Amount,
		CreditAmount:     to.Amount,
		Rate:             rate,
		OperationAt:      from.OperationAt,
		Memo:             from.Memo,
	}); err != nil {
		return fmt.Errorf("create transfer %d/%d: %w", tx.ID, partner.ID, err)
	}

	return nil
}
