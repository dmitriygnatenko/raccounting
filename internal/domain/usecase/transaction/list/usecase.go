// Package list is the ListTransactions use case: it returns every transaction (including transfer
// legs) belonging to the signed-in user.
package list

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ListTransactions.
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

// Execute returns every transaction.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	transactions, err := uc.transactionRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list transactions: load", "error", err)

		return Output{}, errors.New("Failed to load transactions")
	}

	if transactions == nil {
		transactions = []entity.Transaction{}
	}

	return Output{Transactions: transactions}, nil
}
