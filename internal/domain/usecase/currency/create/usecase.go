// Package create is the CreateCurrency use case.
package create

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements CreateCurrency.
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

// Execute validates and creates a new currency, rejecting a code the user already has.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "create currency: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	code := strings.ToUpper(strings.TrimSpace(input.Code))
	name := strings.TrimSpace(input.Name)

	currency, err := uc.currencyRepository.Create(ctx, port.CurrencyCreateRequest{
		Code:    code,
		Symbol:  input.Symbol,
		Name:    name,
		Rate:    input.Rate,
		Default: input.Default,
	})
	if err != nil {
		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "This currency code already exists"}
		}

		slog.ErrorContext(ctx, "create currency: save", "error", err)

		return Output{}, errors.New("Failed to save currency")
	}

	return Output{Currency: currency}, nil
}
