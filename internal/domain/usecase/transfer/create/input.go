package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what CreateTransfer needs to move money between two of the signed-in user's accounts.
// ToAmount/Rate are optional: a same-currency transfer typically omits them, in which case ToAmount
// defaults to Amount and Rate to 1 — mirrors App.api.createTransfer in the frontend mock.
type Input struct {
	FromAccountID uint64
	ToAccountID   uint64
	Amount        int64
	ToAmount      *int64
	Rate          *float64
	Date          string
	Memo          string
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Memo = strings.TrimSpace(i.Memo)

	return validation.ValidateStruct(&i,
		validation.Field(&i.FromAccountID, usecase.AccountIDRules()...),
		validation.Field(&i.ToAccountID, usecase.AccountIDRules()...),
		validation.Field(&i.Amount, validation.Min(1).Error(
			"Transfer amount must be greater than zero")),
		validation.Field(&i.ToAmount, validation.When(i.ToAmount != nil,
			validation.Min(int64(1)).Error("Transfer amount must be greater than zero"),
		)),
		validation.Field(&i.Date, usecase.DateRules()...),
	)
}
