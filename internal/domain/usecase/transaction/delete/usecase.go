// Package delete is the DeleteTransaction use case: it removes an expense/income transaction and
// reverses its balance effect, atomically, in one DB transaction (see port.TransactionRepository).
// A transfer leg can't be deleted this way; delete the whole transfer instead (see transfer/delete).
// It has no output.go — Execute only ever reports success or an error.
package delete

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements DeleteTransaction.
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

// Execute deletes an expense/income transaction, reversing its balance effect.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	existing, err := uc.transactionRepository.FindByID(ctx, input.ID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return &domainError.NotFoundError{Message: "Transaction not found"}
		}

		slog.ErrorContext(ctx, "delete transaction: find", "id", input.ID, "error", err)

		return errors.New("Failed to load transaction")
	}

	if existing.IsTransfer() {
		return &domainError.ValidationError{
			Message: "This is one leg of a transfer — delete the transfer instead",
		}
	}

	if err = uc.transactionRepository.Delete(ctx, input.ID); err != nil {
		if domainError.IsNotFoundError(err) {
			return &domainError.NotFoundError{Message: "Transaction not found"}
		}

		if domainError.IsConflictError(err) {
			return &domainError.ConflictError{Message: "This would overdraw the account"}
		}

		slog.ErrorContext(ctx, "delete transaction: delete", "id", input.ID, "error", err)

		return errors.New("Failed to delete transaction")
	}

	return nil
}
