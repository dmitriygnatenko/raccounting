// Package budget implements port.BudgetRepository on top of the budgets
// table.
package budget

import (
	"context"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
	"raccounting/internal/storage/model"
)

// Storage is the slice of the mysql adapter this repository uses — the budgets table and
// nothing else.
type Storage interface {
	ListCategoryBudgets(ctx context.Context) ([]model.Budget, error)
	// SetCategoryBudget upserts the (categoryId, monthKey) row when Amount > 0, and deletes it
	// otherwise.
	SetCategoryBudget(ctx context.Context, req port.BudgetSetRequest) error
}

// Repository implements port.BudgetRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every category budget.
func (r *Repository) List(ctx context.Context) ([]entity.Budget, error) {
	rows, err := r.storage.ListCategoryBudgets(ctx)
	if err != nil {
		return nil, err
	}

	budgets := make([]entity.Budget, len(rows))
	for i, row := range rows {
		budgets[i] = row.ToEntity()
	}

	return budgets, nil
}

// Set upserts (or, for a non-positive amount, clears) a category's budget for a month.
func (r *Repository) Set(ctx context.Context, req port.BudgetSetRequest) error {
	return r.storage.SetCategoryBudget(ctx, req)
}
