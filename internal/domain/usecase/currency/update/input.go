package update

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what UpdateCurrency needs to change an existing currency's symbol/name/rate/default/
// archived flag. Code is immutable after creation.
type Input struct {
	Code     string
	Symbol   string
	Name     string
	Rate     float64
	Default  bool
	Archived bool
}

// Validate rejects structurally invalid input before any repository lookup. Code is expected to
// already be normalized (uppercase, trimmed) by the caller — see UseCase.Execute.
func (i Input) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	return validation.ValidateStruct(&i,
		validation.Field(&i.Code, usecase.CurrencyCodeRules()...),
		validation.Field(&i.Symbol, usecase.CurrencySymbolRules()...),
		validation.Field(&i.Name, usecase.CurrencyNameRules()...),
		validation.Field(&i.Rate, usecase.RateRules()...),
	)
}
