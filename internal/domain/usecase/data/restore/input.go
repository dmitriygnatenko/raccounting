package restore

import (
	"fmt"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase"
)

// Input is what RestoreData needs to replace all of raccounting's data with a previously exported
// entity.Backup. UserID is whose UI language preference gets applied — raccounting is single-user, so
// every other table isn't scoped to a user at all (see UseCase.wipe/recreate).
type Input struct {
	UserID uint64
	Backup entity.Backup
}

// Validate rejects a structurally or referentially broken backup before UseCase.wipe touches
// anything — once that starts, there's no going back. It checks the format version, every item's own
// fields (reusing the same rule sets create/update use cases validate against), and that every
// cross-reference within the file (account currency, transaction account/category/tags, transfer
// pairing, budget category) actually resolves to something else in the file.
func (i Input) Validate() error {
	b := i.Backup

	if b.Version != entity.BackupFormatVersion {
		return fmt.Errorf(
			"Unsupported backup version %d (this version of raccounting expects %d)",
			b.Version, entity.BackupFormatVersion,
		)
	}

	currencyCodes, err := validateCurrencies(b.Currencies)
	if err != nil {
		return err
	}

	accountIDs, err := validateAccounts(b.Accounts, currencyCodes)
	if err != nil {
		return err
	}

	categoryIDs, err := validateCategories(b.Categories)
	if err != nil {
		return err
	}

	tagIDs, err := validateTags(b.Tags)
	if err != nil {
		return err
	}

	if err := validateTransactions(b.Transactions, accountIDs, categoryIDs, tagIDs); err != nil {
		return err
	}

	if err := validateBudgets(b.Budgets, categoryIDs); err != nil {
		return err
	}

	if b.Settings.Language != "" {
		if err := validation.Validate(b.Settings.Language, usecase.LanguageRules()...); err != nil {
			return fmt.Errorf("Settings: %w", err)
		}
	}

	if b.Settings.Theme != "" {
		if err := validation.Validate(b.Settings.Theme, usecase.ThemeRules()...); err != nil {
			return fmt.Errorf("Settings: %w", err)
		}
	}

	return nil
}

func validateCurrencies(currencies []entity.Currency) (map[string]bool, error) {
	codes := make(map[string]bool, len(currencies))
	defaults := 0

	for _, cur := range currencies {
		if codes[cur.Code] {
			return nil, fmt.Errorf("Duplicate currency code %q", cur.Code)
		}

		codes[cur.Code] = true

		if err := validation.Validate(cur.Code, usecase.CurrencyCodeRules()...); err != nil {
			return nil, fmt.Errorf("Currency %q: %w", cur.Code, err)
		}

		if err := validation.Validate(cur.Symbol, usecase.CurrencySymbolRules()...); err != nil {
			return nil, fmt.Errorf("Currency %q: %w", cur.Code, err)
		}

		if err := validation.Validate(cur.Name, usecase.CurrencyNameRules()...); err != nil {
			return nil, fmt.Errorf("Currency %q: %w", cur.Code, err)
		}

		if err := validation.Validate(cur.Rate, usecase.RateRules()...); err != nil {
			return nil, fmt.Errorf("Currency %q: %w", cur.Code, err)
		}

		if cur.Default {
			defaults++
		}
	}

	if defaults > 1 {
		return nil, fmt.Errorf("More than one currency is marked as default")
	}

	return codes, nil
}

func validateAccounts(accounts []entity.Account, currencyCodes map[string]bool) (map[uint64]bool, error) {
	ids := make(map[uint64]bool, len(accounts))

	for _, acc := range accounts {
		if acc.ID == 0 || ids[acc.ID] {
			return nil, fmt.Errorf("Account %q: missing or duplicate id", acc.Name)
		}

		ids[acc.ID] = true

		if err := validation.Validate(acc.Name, usecase.AccountNameRules()...); err != nil {
			return nil, fmt.Errorf("Account %q: %w", acc.Name, err)
		}

		if err := validation.Validate(uint8(acc.Type), usecase.AccountTypeRules()...); err != nil {
			return nil, fmt.Errorf("Account %q: %w", acc.Name, err)
		}

		if !currencyCodes[acc.CurrencyCode] {
			return nil, fmt.Errorf("Account %q: unknown currency %q", acc.Name, acc.CurrencyCode)
		}
	}

	return ids, nil
}

func validateCategories(categories []entity.Category) (map[uint64]bool, error) {
	ids := make(map[uint64]bool, len(categories))

	for _, cat := range categories {
		if cat.ID == 0 || ids[cat.ID] {
			return nil, fmt.Errorf("Category %q: missing or duplicate id", cat.Name)
		}

		ids[cat.ID] = true

		if err := validation.Validate(cat.Name, usecase.CategoryNameRules()...); err != nil {
			return nil, fmt.Errorf("Category %q: %w", cat.Name, err)
		}

		if err := validation.Validate(uint8(cat.Type), usecase.CategoryTypeRules()...); err != nil {
			return nil, fmt.Errorf("Category %q: %w", cat.Name, err)
		}

		if err := validation.Validate(cat.Color, usecase.CategoryColorRules()...); err != nil {
			return nil, fmt.Errorf("Category %q: %w", cat.Name, err)
		}
	}

	return ids, nil
}

func validateTags(tags []entity.Tag) (map[uint64]bool, error) {
	ids := make(map[uint64]bool, len(tags))

	for _, tag := range tags {
		if tag.ID == 0 || ids[tag.ID] {
			return nil, fmt.Errorf("Tag %q: missing or duplicate id", tag.Name)
		}

		ids[tag.ID] = true

		if err := validation.Validate(tag.Name, usecase.TagNameRules()...); err != nil {
			return nil, fmt.Errorf("Tag %q: %w", tag.Name, err)
		}

		if err := validation.Validate(tag.Color, usecase.TagColorRules()...); err != nil {
			return nil, fmt.Errorf("Tag %q: %w", tag.Name, err)
		}
	}

	return ids, nil
}

func validateTransactions(
	transactions []entity.Transaction,
	accountIDs, categoryIDs, tagIDs map[uint64]bool,
) error {
	byID := make(map[uint64]entity.Transaction, len(transactions))

	for _, tx := range transactions {
		if tx.ID == 0 || byID[tx.ID].ID != 0 {
			return fmt.Errorf("Transaction %d: missing or duplicate id", tx.ID)
		}

		byID[tx.ID] = tx
	}

	for _, tx := range transactions {
		if !accountIDs[tx.AccountID] {
			return fmt.Errorf("Transaction %d: unknown account", tx.ID)
		}

		if len(tx.Memo) > entity.MaxMemoLength {
			return fmt.Errorf("Transaction %d: memo is too long", tx.ID)
		}

		if tx.IsTransfer() {
			if err := validateTransferLeg(tx, byID); err != nil {
				return err
			}

			continue
		}

		if tx.Type != entity.TransactionTypeIncome && tx.Type != entity.TransactionTypeExpense {
			return fmt.Errorf("Transaction %d: unknown type", tx.ID)
		}

		if tx.CategoryID != nil && !categoryIDs[*tx.CategoryID] {
			return fmt.Errorf("Transaction %d: unknown category", tx.ID)
		}

		for _, tagID := range tx.TagIDs {
			if !tagIDs[tagID] {
				return fmt.Errorf("Transaction %d: unknown tag", tx.ID)
			}
		}
	}

	return nil
}

// validateTransferLeg checks tx (one leg of a transfer) against its partner: both must point at each
// other, agree on which two accounts are involved, and carry opposite-signed amounts — exactly what
// UseCase.recreate assumes when it reconstructs the pair via transactionRepository.CreateTransfer.
func validateTransferLeg(tx entity.Transaction, byID map[uint64]entity.Transaction) error {
	if tx.CategoryID != nil {
		return fmt.Errorf("Transaction %d: a transfer leg can't have a category", tx.ID)
	}

	if tx.TransferTransactionID == nil || tx.TransferAccountID == nil {
		return fmt.Errorf("Transaction %d: incomplete transfer data", tx.ID)
	}

	if tx.Amount == 0 {
		return fmt.Errorf("Transaction %d: transfer amount can't be zero", tx.ID)
	}

	partner, ok := byID[*tx.TransferTransactionID]
	if !ok {
		return fmt.Errorf("Transaction %d: transfer partner not found", tx.ID)
	}

	if partner.TransferTransactionID == nil || *partner.TransferTransactionID != tx.ID {
		return fmt.Errorf("Transaction %d: transfer partner doesn't point back", tx.ID)
	}

	if partner.AccountID != *tx.TransferAccountID || tx.AccountID != *partner.TransferAccountID {
		return fmt.Errorf("Transaction %d: transfer accounts don't match", tx.ID)
	}

	if (tx.Amount > 0) == (partner.Amount > 0) {
		return fmt.Errorf("Transaction %d: transfer legs must have opposite signs", tx.ID)
	}

	return nil
}

func validateBudgets(budgets []entity.Budget, categoryIDs map[uint64]bool) error {
	for _, budget := range budgets {
		if budget.Amount <= 0 {
			continue
		}

		if !categoryIDs[budget.CategoryID] {
			return fmt.Errorf("Budget %s: unknown category", budget.MonthKey)
		}

		if err := validation.Validate(budget.MonthKey, usecase.MonthKeyRules()...); err != nil {
			return fmt.Errorf("Budget %s: %w", budget.MonthKey, err)
		}
	}

	return nil
}
