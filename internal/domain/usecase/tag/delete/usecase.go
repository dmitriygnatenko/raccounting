// Package delete is the DeleteTag use case: it removes a tag, detaching it from any transaction
// that had it (see the transaction_tags ON DELETE CASCADE in the adapter migrations — unlike a
// category, nothing blocks a tag delete). It has no output.go — Execute only ever reports success
// or an error.
package delete

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements DeleteTag.
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

// Execute deletes a tag.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	err := uc.tagRepository.Delete(ctx, input.ID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return &domainError.NotFoundError{Message: "Tag not found"}
		}

		slog.ErrorContext(ctx, "delete tag: delete", "id", input.ID, "error", err)

		return errors.New("Failed to delete tag")
	}

	return nil
}
