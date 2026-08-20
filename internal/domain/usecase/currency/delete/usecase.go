// Package delete is the DeleteCurrency use case: it removes a currency, refusing to do so while it
// is still referenced by an account (enforced by the accounts/currencies FK, ON DELETE RESTRICT —
// see the adapter migrations) — mirrors isCurrencyInUse in the frontend's store. It has no
// output.go — Execute only ever reports success or an error.
package delete

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements DeleteCurrency.
type UseCase struct {
	currencyRepository port.CurrencyRepository
}

// New builds a UseCase from its dependencies.
func New(
	currencyRepository port.CurrencyRepository,
) *UseCase {
	return &UseCase{
		currencyRepository: currencyRepository,
	}
}

// Execute deletes a currency, refusing while it's still in use by an account.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	err := uc.currencyRepository.Delete(ctx, input.Code)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return &domainError.NotFoundError{Message: "Currency not found"}
		}

		if domainError.IsConflictError(err) {
			return &domainError.ConflictError{
				Message: "This currency is in use — remove or recode its accounts first",
			}
		}

		slog.ErrorContext(ctx, "delete currency: delete", "code", input.Code, "error", err)

		return errors.New("Failed to delete currency")
	}

	return nil
}
