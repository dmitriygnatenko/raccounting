// Package list is the ListTags use case: it returns every tag.
package list

import (
	"context"
	"errors"
	"log/slog"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

// UseCase implements ListTags.
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

// Execute returns every tag.
func (uc *UseCase) Execute(ctx context.Context) (Output, error) {
	tags, err := uc.tagRepository.List(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "list tags: load", "error", err)

		return Output{}, errors.New("Failed to load tags")
	}

	if tags == nil {
		tags = []entity.Tag{}
	}

	return Output{Tags: tags}, nil
}
