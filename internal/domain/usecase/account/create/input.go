package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what CreateAccount needs to create a new account.
type Input struct {
	Name         string
	Type         string
	CurrencyCode string
	Balance      int64
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	return validation.ValidateStruct(&i,
		validation.Field(&i.Name, usecase.AccountNameRules()...),
		validation.Field(&i.Type, usecase.AccountTypeRules()...),
		validation.Field(&i.CurrencyCode, usecase.CurrencyCodeRules()...),
		validation.Field(&i.Balance, validation.Min(int64(0)).Error("Opening balance can't be negative")),
	)
}
