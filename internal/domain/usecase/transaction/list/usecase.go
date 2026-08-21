// Package list is the ListTransactions use case: it returns one page of transactions belonging to
// the signed-in user, matching an optional date range and set of filters (account, category, tag,
// type, memo/category-name search).
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

// Execute returns one page of transactions matching input.
func (uc *UseCase) Execute(ctx context.Context, input Input) (Output, error) {
	input = input.normalize()

	result, err := uc.transactionRepository.ListFiltered(ctx, port.TransactionListFilter{
		DateFrom:   input.DateFrom,
		DateTo:     input.DateTo,
		AccountID:  input.AccountID,
		CategoryID: input.CategoryID,
		TagID:      input.TagID,
		Type:       input.Type,
		Search:     input.Search,
		Page:       input.Page,
		PageSize:   input.PageSize,
	})
	if err != nil {
		slog.ErrorContext(ctx, "list transactions: load", "error", err)

		return Output{}, errors.New("Failed to load transactions")
	}

	transactions := result.Transactions
	if transactions == nil {
		transactions = []entity.Transaction{}
	}

	sums := result.SumsByCurrency
	if sums == nil {
		sums = map[string]int64{}
	}

	totalPages := max((result.TotalCount+input.PageSize-1)/input.PageSize, 1)

	return Output{
		Transactions:   transactions,
		Page:           input.Page,
		PageSize:       input.PageSize,
		TotalCount:     result.TotalCount,
		TotalPages:     totalPages,
		SumsByCurrency: sums,
	}, nil
}
