// Package create is the CreateTransfer use case: it validates both accounts exist, then inserts two
// linked transaction rows pointing at each other's id via transfer_transaction_id (debit leg
// negative, credit leg positive, type "transfer", no category) and updates both account balances —
// all atomically, in one DB transaction (see port.TransactionRepository.CreateTransfer). Mirrors
// App.api.createTransfer in the frontend.
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

// UseCase implements CreateTransfer.
type UseCase struct {
	transactionRepository port.TransactionRepository
	accountRepository     port.AccountRepository
}

// New builds a UseCase from its dependencies.
func New(
	transactionRepository port.TransactionRepository,
	accountRepository port.AccountRepository,
) *UseCase {
	return &UseCase{
		transactionRepository: transactionRepository,
		accountRepository:     accountRepository,
	}
}

// Execute validates and creates a transfer between two accounts.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "create transfer: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	if input.FromAccountID == input.ToAccountID {
		return Output{}, &domainError.ValidationError{Message: "Choose two different accounts"}
	}

	fromAccount, err := uc.accountRepository.FindByID(ctx, input.FromAccountID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.ValidationError{Message: "Source account not found"}
		}

		slog.ErrorContext(ctx, "create transfer: verify source account", "error", err)

		return Output{}, errors.New("Failed to verify source account")
	}

	toAccount, err := uc.accountRepository.FindByID(ctx, input.ToAccountID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.ValidationError{Message: "Destination account not found"}
		}

		slog.ErrorContext(ctx, "create transfer: verify destination account", "error", err)

		return Output{}, errors.New("Failed to verify destination account")
	}

	creditAmount := input.Amount
	if input.ToAmount != nil {
		creditAmount = *input.ToAmount
	}

	rate := 1.0
	if input.Rate != nil {
		rate = *input.Rate
	}

	operationAt, err := time.Parse(entity.DateLayout, input.Date)
	if err != nil {
		return Output{}, &domainError.ValidationError{Message: "Date must be in YYYY-MM-DD format"}
	}

	memo := strings.TrimSpace(input.Memo)

	result, err := uc.transactionRepository.CreateTransfer(ctx, port.TransferCreateRequest{
		FromAccountID:    input.FromAccountID,
		FromCurrencyCode: fromAccount.CurrencyCode,
		ToAccountID:      input.ToAccountID,
		ToCurrencyCode:   toAccount.CurrencyCode,
		Amount:           input.Amount,
		CreditAmount:     creditAmount,
		Rate:             rate,
		OperationAt:      operationAt,
		Memo:             memo,
	})
	if err != nil {
		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "This would overdraw the account"}
		}

		slog.ErrorContext(ctx, "create transfer: save", "error", err)

		return Output{}, errors.New("Failed to save transfer")
	}

	return Output{
		LegFrom: result.LegFrom,
		LegTo:   result.LegTo,
	}, nil
}
