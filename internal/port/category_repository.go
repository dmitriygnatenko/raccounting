package port

//go:generate go tool mockgen -source=category_repository.go -destination=mocks/category_repository_mock.go -package=mocks

import (
	"context"

	"raccounting/internal/domain/entity"
)

// CategoryCreateRequest bundles the CategoryRepository.Create parameters.
type CategoryCreateRequest struct {
	Name  string
	Type  entity.CategoryType
	Color string
}

// CategoryUpdateRequest bundles the CategoryRepository.Update parameters.
type CategoryUpdateRequest struct {
	ID       uint64
	Name     string
	Color    string
	Archived bool
}

// CategoryRepository persists Categories.
type CategoryRepository interface {
	List(ctx context.Context) ([]entity.Category, error)
	// Exists reports whether a category with this id exists — used to validate a transaction's
	// category before saving it.
	Exists(ctx context.Context, id uint64) (bool, error)
	Create(ctx context.Context, req CategoryCreateRequest) (entity.Category, error)
	// Update returns a *domainerror.NotFoundError if no category with this id exists. Type is
	// immutable after creation (matches the frontend, which never lets a category switch between
	// expense/income).
	Update(ctx context.Context, req CategoryUpdateRequest) (entity.Category, error)
	// Delete returns a *domainerror.NotFoundError if no category with this id exists, or a
	// *domainerror.ConflictError if it is still referenced by a transaction.
	Delete(ctx context.Context, id uint64) error
}
