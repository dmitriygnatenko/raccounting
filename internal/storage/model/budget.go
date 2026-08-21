package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Budget is the shape of a row in the budgets table.
type Budget struct {
	ID         uint64
	MonthKey   string
	CategoryID uint64
	Amount     int64
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ToEntity converts the stored row into a domain entity.Budget.
func (m Budget) ToEntity() entity.Budget {
	return entity.Budget{
		ID:         m.ID,
		MonthKey:   m.MonthKey,
		CategoryID: m.CategoryID,
		Amount:     m.Amount,
		CreatedAt:  m.CreatedAt,
		UpdatedAt:  m.UpdatedAt,
	}
}

// BudgetSetRequest bundles the parameters Storage.SetCategoryBudget needs. Set upserts the
// (categoryId, monthKey) row when Amount > 0, and deletes it otherwise.
type BudgetSetRequest struct {
	CategoryID uint64
	MonthKey   string
	Amount     int64
}
