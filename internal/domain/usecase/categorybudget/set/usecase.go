// Package set is the SetCategoryBudget use case: it plans (or clears) how much is budgeted for a
// category in a given month. It has no output.go — Execute only ever reports success or an error.
package set

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements SetCategoryBudget.
type UseCase struct {
	categoryBudgetRepository port.BudgetRepository
	categoryRepository       port.CategoryRepository
}

// New builds a UseCase from its dependencies.
func New(
	categoryBudgetRepository port.BudgetRepository,
	categoryRepository port.CategoryRepository,
) *UseCase {
	return &UseCase{
		categoryBudgetRepository: categoryBudgetRepository,
		categoryRepository:       categoryRepository,
	}
}

// Execute sets (or, for a non-positive amount, clears) a category's budget for a month.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "set category budget: validation", "error", err)

		return domainError.ToValidationError(err)
	}

	exists, err := uc.categoryRepository.Exists(ctx, input.CategoryID)
	if err != nil {
		slog.ErrorContext(ctx, "set category budget: verify category", "error", err)

		return errors.New("Failed to verify category")
	}

	if !exists {
		return &domainError.ValidationError{Message: "Category not found"}
	}

	if err = uc.categoryBudgetRepository.Set(ctx, port.BudgetSetRequest{
		CategoryID: input.CategoryID,
		MonthKey:   input.MonthKey,
		Amount:     input.Amount,
	}); err != nil {
		slog.ErrorContext(ctx, "set category budget: save", "error", err)

		return errors.New("Failed to save category budget")
	}

	return nil
}
