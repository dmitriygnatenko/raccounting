// Package update is the UpdateTransaction use case: it changes an existing expense/income
// transaction, reversing its old balance effect and applying the new one, atomically, in one DB
// transaction (see port.TransactionRepository) — even when its account changed. A transfer leg
// can't be edited this way; delete the transfer instead (see transfer/delete).
package update

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

// UseCase implements UpdateTransaction.
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

// Execute updates an existing expense/income transaction.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update transaction: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	existing, err := uc.transactionRepository.FindByID(ctx, input.ID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.NotFoundError{Message: "Transaction not found"}
		}

		slog.ErrorContext(ctx, "update transaction: find", "id", input.ID, "error", err)

		return Output{}, errors.New("Failed to load transaction")
	}

	if existing.IsTransfer() {
		return Output{}, &domainError.ValidationError{
			Message: "This is one leg of a transfer — delete the transfer instead of editing it",
		}
	}

	account, err := uc.accountRepository.FindByID(ctx, input.AccountID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.ValidationError{Message: "Account not found"}
		}

		slog.ErrorContext(ctx, "update transaction: verify account", "error", err)

		return Output{}, errors.New("Failed to verify account")
	}

	if input.CategoryID != nil {
		exists, existsErr := uc.categoryRepository.Exists(ctx, *input.CategoryID)
		if existsErr != nil {
			slog.ErrorContext(ctx, "update transaction: verify category", "error", existsErr)

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

	tx, err := uc.transactionRepository.Update(ctx, port.TransactionUpdateRequest{
		ID:           input.ID,
		AccountID:    input.AccountID,
		CategoryID:   input.CategoryID,
		Type:         entity.TransactionType(input.Type),
		CurrencyCode: account.CurrencyCode,
		Amount:       input.Amount,
		Memo:         strings.TrimSpace(input.Memo),
		OperationAt:  operationAt,
	})
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.NotFoundError{Message: "Transaction not found"}
		}

		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "This would overdraw the account"}
		}

		slog.ErrorContext(ctx, "update transaction: save", "id", input.ID, "error", err)

		return Output{}, errors.New("Failed to update transaction")
	}

	return Output{Transaction: tx}, nil
}
