// Package update is the UpdateTag use case: it renames/recolors an existing tag.
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

// UseCase implements UpdateTag.
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

// Execute renames/recolors an existing tag, rejecting a name another tag already has.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) (Output, error) {
	if err := input.Validate(); err != nil {
		slog.InfoContext(ctx, "update tag: validation", "error", err)

		return Output{}, domainError.ToValidationError(err)
	}

	name := strings.TrimSpace(input.Name)
	color := usecase.ResolveColor(input.Color)

	tag, err := uc.tagRepository.Update(ctx, port.TagUpdateRequest{
		ID:    input.ID,
		Name:  name,
		Color: color,
	})
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return Output{}, &domainError.NotFoundError{Message: "Tag not found"}
		}

		if domainError.IsConflictError(err) {
			return Output{}, &domainError.ConflictError{Message: "This tag already exists"}
		}

		slog.ErrorContext(ctx, "update tag: save", "id", input.ID, "error", err)

		return Output{}, errors.New("Failed to update tag")
	}

	return Output{Tag: tag}, nil
}
