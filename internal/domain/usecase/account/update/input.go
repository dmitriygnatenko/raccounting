package update

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what UpdateAccount needs to rename/recolor/archive an existing account. Balance is
// deliberately absent — see entity.Account.
type Input struct {
	ID           uint64
	Name         string
	Type         string
	CurrencyCode string
	Archived     bool
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Name = strings.TrimSpace(i.Name)

	return validation.ValidateStruct(&i,
		validation.Field(&i.Name, usecase.AccountNameRules()...),
		validation.Field(&i.Type, usecase.AccountTypeRules()...),
		validation.Field(&i.CurrencyCode, usecase.CurrencyCodeRules()...),
	)
}
