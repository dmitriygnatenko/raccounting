// Package create is the CreateTransaction use case: it records a new expense or income and adjusts
// its account's balance, atomically, in one DB transaction (see port.TransactionRepository).
package create

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"time"

	"raccounting/internal/domain/entity"
	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements CreateTransaction.
type UseCase struct {
	transactionRepository port.TransactionRepository
	accountRepository     port.AccountRepository
	categoryRepository    port.CategoryRepository
}

// New builds a UseCase from its dependencies.
func New(
	transactionRepository port.TransactionRepository,
	accountRepository port.AccountRepository,
	categoryRepository port.CategoryRepository,
) *UseCase {
	return &UseCase{
		transactionRepository: transactionRepository,
		accountRepository:     accountRepository,
		categoryRepository:    categoryRepository,
	}
}

// Execute validates and creates a new expense/income transaction, verifying its account and
// (if given) category exist first.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "create transaction: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	account, err := uc.accountRepository.FindByID(ctx, input.AccountID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.ValidationError{Message: "Account not found"}
		}

		slog.ErrorContext(ctx, "create transaction: verify account", "error", err)

		return Output{}, errors.New("Failed to verify account")
	}

	if input.CategoryID != nil {
		exists, err := uc.categoryRepository.Exists(ctx, *input.CategoryID)
		if err != nil {
			slog.ErrorContext(ctx, "create transaction: verify category", "error", err)

			return Output{}, errors.New("Failed to verify category")
		}

		if !exists {
			return Output{}, &domainError.ValidationError{Message: "Category not found"}
		}
	}

	operationAt, err := time.Parse(entity.DateLayout, input.Date)
	if err != nil {
		return Output{}, &domainError.ValidationError{Message: "Date must be in YYYY-MM-DD format"}
	}

	transactionType, _ := entity.ParseTransactionType(input.Type)

	tx, err := uc.transactionRepository.Create(ctx, port.TransactionCreateRequest{
		AccountID:    input.AccountID,
		CategoryID:   input.CategoryID,
		Type:         transactionType,
		CurrencyCode: account.CurrencyCode,
		Amount:       input.Amount,
		Memo:         strings.TrimSpace(input.Memo),
		OperationAt:  operationAt,
	})
	if err != nil {
		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "This would overdraw the account"}
		}

		slog.ErrorContext(ctx, "create transaction: save", "error", err)

		return Output{}, errors.New("Failed to save transaction")
	}

	return Output{Transaction: tx}, nil
}
