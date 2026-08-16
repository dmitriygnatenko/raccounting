package set

import (
	validation "github.com/go-ozzo/ozzo-validation/v4"

	"raccounting/internal/domain/usecase"
)

// Input is what SetCategoryBudget needs to plan (or clear) a category's budget for a month. An
// Amount <= 0 clears the budget for that month — mirrors App.api.setCategoryBudget in the frontend
// mock exactly.
type Input struct {
	CategoryID uint64
	MonthKey   string
	Amount     int64
}

// Validate rejects structurally invalid input before any repository lookup.
func (i Input) Validate() error {
	return validation.ValidateStruct(&i,
		validation.Field(&i.CategoryID, usecase.CategoryIDRules()...),
		validation.Field(&i.MonthKey, usecase.MonthKeyRules()...),
	)
}
