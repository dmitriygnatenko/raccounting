// Package list is the ListCategoryBudgets use case: it returns every category budget belonging to
// the signed-in user. The HTTP handler reshapes the flat list into the nested
// {categoryId: {monthKey: amount}} map the frontend expects.
package list

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ListCategoryBudgets.
type UseCase struct {
	categoryBudgetRepository port.BudgetRepository
}

// New builds a UseCase from its dependencies.
func New(
	categoryBudgetRepository port.BudgetRepository,
) *UseCase {
	return &UseCase{
		categoryBudgetRepository: categoryBudgetRepository,
	}
}

// Execute returns every category budget.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	budgets, err := uc.categoryBudgetRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list category budgets: load", "error", err)

		return Output{}, errors.New("Failed to load category budgets")
	}

	if budgets == nil {
		budgets = []entity.Budget{}
	}

	return Output{CategoryBudgets: budgets}, nil
}
