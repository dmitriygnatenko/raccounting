// Package delete is the DeleteCategory use case: it removes a category, refusing to do so while it
// is still referenced by a transaction (enforced by the categories/transactions FK, ON DELETE
// RESTRICT — see the adapter migrations); any budgets set for it are cascade-deleted along with it.
// It has no output.go — Execute only ever reports success or an error.
package delete

import (
	"context"
	"errors"
	"log/slog"

	domainError "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// UseCase implements DeleteCategory.
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

// Execute deletes a category, refusing while it's still in use by a transaction.
func (uc *UseCase) Execute(
	ctx context.Context,
	input Input,
) error {
	err := uc.categoryRepository.Delete(ctx, input.ID)
	if err != nil {
		if domainError.IsNotFoundError(err) {
			return &domainError.NotFoundError{Message: "Category not found"}
		}

		if domainError.IsConflictError(err) {
			return &domainError.ConflictError{Message: "This category is in use — remove its transactions first"}
		}

		slog.ErrorContext(ctx, "delete category: delete", "id", input.ID, "error", err)

		return errors.New("Failed to delete category")
	}

	return nil
}
