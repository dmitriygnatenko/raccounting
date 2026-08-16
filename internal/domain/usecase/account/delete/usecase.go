// Package delete is the DeleteAccount use case: it removes an account, refusing to do so while it
// is still referenced by a transaction (enforced by the accounts/transactions FK, ON DELETE
// RESTRICT — see the adapter migrations) — mirrors isAccountInUse in the frontend's store. It has no
// output.go — Execute only ever reports success or an error.
package delete

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements DeleteAccount.
type UseCase struct {
	accountRepository port.AccountRepository
}

// New builds a UseCase from its dependencies.
func New(
	accountRepository port.AccountRepository,
) *UseCase {
	return &UseCase{
		accountRepository: accountRepository,
	}
}

// Execute deletes an account, refusing while it's still in use by a transaction.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	err := uc.accountRepository.Delete(ctx, input.ID)
	if err != nil {
		if domainError.IsNotFoundError(err) || domainError.IsConflictError(err) {
			return err
		}

		slog.ErrorContext(ctx, "delete account: delete", "id", input.ID, "error", err)

		return errors.New("Failed to delete account")
	}

	return nil
}
