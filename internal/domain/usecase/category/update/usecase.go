// Package update is the UpdateCategory use case: it renames/recolors/archives an existing category.
package update

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/domain/usecase"
	"raccounting/internal/port"
)

// UseCase implements UpdateCategory.
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

// Execute renames/recolors/archives an existing category.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update category: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)
	color := usecase.ResolveColor(input.Color)

	category, err := uc.categoryRepository.Update(ctx, port.CategoryUpdateRequest{
		ID:       input.ID,
		Name:     name,
		Color:    color,
		Archived: input.Archived,
	})
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.NotFoundError{Message: "Category not found"}
		}

		slog.ErrorContext(ctx, "update category: save", "id", input.ID, "error", err)

		return Output{}, errors.New("Failed to update category")
	}

	return Output{Category: category}, nil
}
