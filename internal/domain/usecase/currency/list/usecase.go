// Package list is the ListCurrencies use case: it returns every currency.
package list

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ListCurrencies.
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

// Execute returns every currency.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	currencies, err := uc.currencyRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list currencies: load", "error", err)

		return Output{}, errors.New("Failed to load currencies")
	}

	if currencies == nil {
		currencies = []entity.Currency{}
	}

	return Output{Currencies: currencies}, nil
}
