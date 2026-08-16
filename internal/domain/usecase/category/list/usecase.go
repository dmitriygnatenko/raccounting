// Package list is the ListCategories use case: it returns every category belonging to the signed-in
// user.
package list

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ListCategories.
type UseCase struct {
	categoryRepository port.CategoryRepository
}

// New builds a UseCase from its dependencies.
func New(
	categoryRepository port.CategoryRepository,
) *UseCase {
	return &UseCase{
		categoryRepository: categoryRepository,
	}
}

// Execute returns every category.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	categories, err := uc.categoryRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list categories: load", "error", err)

		return Output{}, errors.New("Failed to load categories")
	}

	if categories == nil {
		categories = []entity.Category{}
	}

	return Output{Categories: categories}, nil
}
