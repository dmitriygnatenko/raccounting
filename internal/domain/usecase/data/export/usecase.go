// Package export is the ExportData use case: it gathers every table raccounting stores into one
// entity.Backup snapshot, for download as a backup file. See the restore package for the
// counterpart that reads one back in.
package export

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ExportData.
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

// Execute gathers every table into one entity.Backup snapshot, carrying userID's saved UI language
// along as Settings.
func (uc *UseCase) Execute(ctx context.Context, userID uint64) (entity.Backup, error) {
	settings, err := uc.userRepository.GetSettings(ctx, userID)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load settings", "error", err)

		return entity.Backup{}, errors.New("Failed to load settings")
	}

	currencies, err := uc.currencyRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load currencies", "error", err)

		return entity.Backup{}, errors.New("Failed to load currencies")
	}

	accounts, err := uc.accountRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load accounts", "error", err)

		return entity.Backup{}, errors.New("Failed to load accounts")
	}

	categories, err := uc.categoryRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load categories", "error", err)

		return entity.Backup{}, errors.New("Failed to load categories")
	}

	tags, err := uc.tagRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load tags", "error", err)

		return entity.Backup{}, errors.New("Failed to load tags")
	}

	transactions, err := uc.transactionRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load transactions", "error", err)

		return entity.Backup{}, errors.New("Failed to load transactions")
	}

	budgets, err := uc.categoryBudgetRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "export data: load budgets", "error", err)

		return entity.Backup{}, errors.New("Failed to load budgets")
	}

	return entity.Backup{
		Version:      entity.BackupFormatVersion,
		ExportedAt:   time.Now(),
		Settings:     settings,
		Currencies:   emptyIfNil(currencies),
		Accounts:     emptyIfNil(accounts),
		Categories:   emptyIfNil(categories),
		Tags:         emptyIfNil(tags),
		Transactions: emptyIfNil(transactions),
		Budgets:      emptyIfNil(budgets),
	}, nil
}

// emptyIfNil turns a nil slice into an empty one, so the exported JSON always has "[]" rather than
// "null" for an empty table.
func emptyIfNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}

	return s
}
