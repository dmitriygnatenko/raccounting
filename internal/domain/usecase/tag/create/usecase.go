// Package create is the CreateTag use case.
package create

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/domain/usecase"
	"raccounting/internal/port"
)

// UseCase implements CreateTag.
type UseCase struct {
	tagRepository port.TagRepository
}

// New builds a UseCase from its dependencies.
func New(
	tagRepository port.TagRepository,
) *UseCase {
	return &UseCase{
		tagRepository: tagRepository,
	}
}

// Execute validates and creates a new tag, rejecting a name the user already has.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "create tag: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)
	color := usecase.ResolveColor(input.Color)

	tag, err := uc.tagRepository.Create(ctx, port.TagCreateRequest{
		Name:  name,
		Color: color,
	})
	if err != nil {
		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "This tag already exists"}
		}

		slog.ErrorContext(ctx, "create tag: save", "error", err)

		return Output{}, errors.New("Failed to save tag")
	}

	return Output{Tag: tag}, nil
}
