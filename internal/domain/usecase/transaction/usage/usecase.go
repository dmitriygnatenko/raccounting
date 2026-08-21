// Package usage is the TransactionUsage use case: it reports which accounts/categories are
// referenced by at least one transaction, so the frontend can gate "delete this account/category"
// without loading every transaction (mirrors isAccountInUse/isCategoryInUse in the frontend store).
package usage

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/port"
)

// UseCase implements TransactionUsage.
type UseCase struct {
	transactionRepository port.TransactionRepository
}

// New builds a UseCase from its dependencies.
func New(
	transactionRepository port.TransactionRepository,
) *UseCase {
	return &UseCase{
		transactionRepository: transactionRepository,
	}
}

// Execute returns the account/category ids referenced by at least one transaction.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	result, err := uc.transactionRepository.Usage(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "transaction usage: load", "error", err)

		return Output{}, errors.New("Failed to load transaction usage")
	}

	accountIDs, categoryIDs := result.AccountIDs, result.CategoryIDs
	if accountIDs == nil {
		accountIDs = []uint64{}
	}

	if categoryIDs == nil {
		categoryIDs = []uint64{}
	}

	return Output{AccountIDs: accountIDs, CategoryIDs: categoryIDs}, nil
}
