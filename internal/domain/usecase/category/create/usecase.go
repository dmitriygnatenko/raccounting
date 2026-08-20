// Package create is the CreateCategory use case.
package create

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"raccounting/internal/domain/entity"
	domainError "raccounting/internal/domain/error"
	"raccounting/internal/domain/usecase"
	"raccounting/internal/port"
)

// UseCase implements CreateCategory.
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

// Execute validates and creates a new category.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "create category: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)
	color := usecase.ResolveColor(input.Color)

	category, err := uc.categoryRepository.Create(ctx, port.CategoryCreateRequest{
		Name:  name,
		Type:  entity.CategoryType(input.Type),
		Color: color,
	})
	if err != nil {
		slog.ErrorContext(ctx, "create category: save", "error", err)

		return Output{}, errors.New("Failed to save category")
	}

	return Output{Category: category}, nil
}
