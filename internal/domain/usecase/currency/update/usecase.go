// Package update is the UpdateCurrency use case: it changes an existing currency's symbol, name,
// exchange rate, or archived flag. Its code is immutable after creation.
package update

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements UpdateCurrency.
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

// Execute updates an existing currency.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	input.Code = strings.ToUpper(strings.TrimSpace(input.Code))

	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update currency: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)

	currency, err := uc.currencyRepository.Update(ctx, port.CurrencyUpdateRequest{
		Code:     input.Code,
		Symbol:   input.Symbol,
		Name:     name,
		Rate:     input.Rate,
		Default:  input.Default,
		Archived: input.Archived,
	})
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, err
		}

		slog.ErrorContext(ctx, "update currency: save", "code", input.Code, "error", err)

		return Output{}, errors.New("Failed to update currency")
	}

	return Output{Currency: currency}, nil
}
