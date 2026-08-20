package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase"
)

// Input is what CreateAccount needs to create a new account. Type is the bare numeric
// entity.AccountType value — see entity.AccountTypes.
type Input struct {
	Name         string
	Type         uint8
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
		validation.Field(&i.Balance, i.balanceRules()...),
	)
}

// balanceRules allows a negative opening balance for credit card and debt accounts, where the
// balance represents money owed rather than money held. Every other account type must open
// non-negative.
func (i Input) balanceRules() []validation.Rule {
	t := entity.AccountType(i.Type)
	if t == entity.AccountTypeCreditCard || t == entity.AccountTypeDebt {
		return nil
	}

	return []validation.Rule{
		validation.Min(int64(0)).Error("Opening balance can't be negative"),
	}
}
