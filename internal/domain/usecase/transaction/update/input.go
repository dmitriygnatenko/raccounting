package update

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase"
)

// Input is what UpdateTransaction needs to change an existing expense/income transaction. Type is
// the bare numeric entity.TransactionType value. Amount is signed: negative for an expense,
// positive for income.
type Input struct {
	ID         uint64
	AccountID  uint64
	CategoryID *uint64
	Type       uint8
	Memo       string
	Amount     int64
	Date       string
	TagIDs     []uint64
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Memo = strings.TrimSpace(i.Memo)

	return validation.ValidateStruct(&i,
		validation.Field(&i.AccountID, usecase.AccountIDRules()...),
		validation.Field(&i.Type,
			validation.Required.Error("Transaction type must be \"expense\" or \"income\""),
			validation.In(uint8(entity.TransactionTypeExpense), uint8(entity.TransactionTypeIncome)).
				Error("Transaction type must be \"expense\" or \"income\""),
		),
		validation.Field(&i.Memo, validation.Length(0, entity.MaxMemoLength).
			Error("Memo must be at most 1000 characters")),
		validation.Field(&i.Amount, usecase.AmountRules()...),
		validation.Field(&i.Date, usecase.DateRules()...),
	)
}
