package port

import (
	"context"

	"raccounting/internal/domain/entity"
)

// BudgetSetRequest bundles the BudgetRepository.Set parameters. Set upserts the
// (categoryId, monthKey) row when Amount > 0, and deletes it otherwise — mirrors
// App.api.setCategoryBudget in the frontend.
type BudgetSetRequest struct {
	CategoryID uint64
	MonthKey   string
	Amount     int64
}

// BudgetRepository persists Budgets
type BudgetRepository interface {
	List(ctx context.Context) ([]entity.Budget, error)
	Set(ctx context.Context, req BudgetSetRequest) error
}
