// Package delete is the DeleteTransfer use case: it removes both legs of a transfer and reverses
// their balance effects, atomically, in one DB transaction (see
// port.TransactionRepository.DeleteTransfer). Mirrors App.api.deleteTransfer in the frontend mock.
// It has no output.go — Execute only ever reports success or an error.
package delete

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements DeleteTransfer.
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

// Execute deletes both legs of a transfer, reversing their balance effects.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	found, err := uc.transactionRepository.DeleteTransfer(ctx, input.ID)
	if err != nil {
		if domainError.IsConflictError(err) {
			return &domainError.ConflictError{Message: "This would overdraw the account"}
		}

		slog.ErrorContext(ctx, "delete transfer: delete", "id", input.ID, "error", err)

		return errors.New("Failed to delete transfer")
	}

	if !found {
		return &domainError.NotFoundError{Message: "Transfer not found"}
	}

	return nil
}
