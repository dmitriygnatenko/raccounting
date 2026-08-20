package port

//go:generate go tool mockgen -source=budget_repository.go -destination=mocks/budget_repository_mock.go -package=mocks

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

// BudgetRepository persists Budgets.
type BudgetRepository interface {
	// List returns every category's budget, across all months.
	List(ctx context.Context) ([]entity.Budget, error)
	// Set upserts the (CategoryID, MonthKey) row when Amount > 0, and deletes it otherwise.
	Set(ctx context.Context, req BudgetSetRequest) error
}
