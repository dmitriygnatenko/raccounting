package create

import (
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase"
)

// Input is what CreateTransaction needs to record a new expense/income. Transfers are created
// through the dedicated transfer/create use case instead — Type here is restricted to
// expense/income (see Validate). Amount is signed: negative for an expense, positive for income.
type Input struct {
	AccountID  uint64
	CategoryID *uint64
	Type       string
	Memo       string
	Amount     int64
	Date       string
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	i.Memo = strings.TrimSpace(i.Memo)

	return validation.ValidateStruct(&i,
		validation.Field(&i.AccountID, usecase.AccountIDRules()...),
		validation.Field(&i.Type,
			validation.Required.Error("Transaction type must be \"expense\" or \"income\""),
			validation.In(entity.TransactionTypeExpense.String(), entity.TransactionTypeIncome.String()).
				Error("Transaction type must be \"expense\" or \"income\""),
		),
		validation.Field(&i.Memo, validation.Length(0, entity.MaxMemoLength).
			Error("Memo must be at most 1000 characters")),
		validation.Field(&i.Date, usecase.DateRules()...),
	)
}
