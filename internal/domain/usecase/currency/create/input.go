package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what CreateCurrency needs to create a new currency.
type Input struct {
	Code    string
	Symbol  string
	Name    string
	Rate    float64
	Default bool
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Code = strings.ToUpper(strings.TrimSpace(i.Code))

	return validation.ValidateStruct(&i,
		validation.Field(&i.Code, usecase.CurrencyCodeRules()...),
		validation.Field(&i.Symbol, usecase.CurrencySymbolRules()...),
		validation.Field(&i.Name, usecase.CurrencyNameRules()...),
		validation.Field(&i.Rate, usecase.RateRules()...),
	)
}
