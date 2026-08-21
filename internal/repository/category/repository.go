// Package category implements port.CategoryRepository on top of the categories table.
package category

import (
	"context"
	"errors"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

//go:generate go tool mockgen -source=repository.go -destination=mocks/storage_mock.go -package=mocks

// Storage is the slice of the DB adapter this repository uses — the categories table and nothing
// else.
type Storage interface {
	ListCategories(ctx context.Context) ([]model.Category, error)
	// ExistsCategory reports whether a category with this id exists.
	ExistsCategory(ctx context.Context, id uint64) (bool, error)
	CreateCategory(ctx context.Context, req model.CategoryCreateRequest) (id uint64, err error)
	// UpdateCategory changes name/color/status, returning the full updated row. found is false if
	// no category with this id exists.
	UpdateCategory(
		ctx context.Context,
		req model.CategoryUpdateRequest,
	) (row model.Category, found bool, err error)
	// DeleteCategory removes a category row. found is false if no category with this id existed. A
	// FOREIGN KEY violation (the category is still referenced by a transaction) comes back wrapped
	// in storageError.ForeignKeyViolationError.
	DeleteCategory(ctx context.Context, id uint64) (found bool, err error)
}

// Repository implements port.CategoryRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every category.
func (r *Repository) List(ctx context.Context) ([]entity.Category, error) {
	rows, err := r.storage.ListCategories(ctx)
	if err != nil {
		return nil, err
	}

	categories := make([]entity.Category, len(rows))
	for i, row := range rows {
		categories[i] = row.ToEntity()
	}

	return categories, nil
}

// Exists reports whether a category with this id exists.
func (r *Repository) Exists(ctx context.Context, id uint64) (bool, error) {
	return r.storage.ExistsCategory(ctx, id)
}

// Create inserts a new category and returns it.
func (r *Repository) Create(
	ctx context.Context, req port.CategoryCreateRequest,
) (entity.Category, error) {
	id, err := r.storage.CreateCategory(ctx, model.CategoryCreateRequest{
		Name:  req.Name,
		Type:  req.Type,
		Color: req.Color,
	})
	if err != nil {
		return entity.Category{}, err
	}

	return entity.Category{
		ID:     id,
		Name:   req.Name,
		Type:   req.Type,
		Color:  req.Color,
		Status: entity.CategoryStatusActive,
	}, nil
}

// Update changes an existing category's name/color/archived flag. An unknown id is reported as a
// message-less *domainerror.NotFoundError — the use case supplies the message.
func (r *Repository) Update(
	ctx context.Context, req port.CategoryUpdateRequest,
) (entity.Category, error) {
	row, found, err := r.storage.UpdateCategory(ctx, model.CategoryUpdateRequest{
		ID:       req.ID,
		Name:     req.Name,
		Color:    req.Color,
		Archived: req.Archived,
	})
	if err != nil {
		return entity.Category{}, err
	}

	if !found {
		return entity.Category{}, &domainerror.NotFoundError{}
	}

	return row.ToEntity(), nil
}

// Delete removes a category, reporting a *domainerror.NotFoundError if it doesn't exist, or a
// *domainerror.ConflictError if it's still referenced by a transaction. Both are message-less — the
// use case supplies the message.
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	found, err := r.storage.DeleteCategory(ctx, id)
	if err != nil {
		if errors.Is(err, storageError.ForeignKeyViolationError) {
			return &domainerror.ConflictError{}
		}

		return err
	}

	if !found {
		return &domainerror.NotFoundError{}
	}

	return nil
}
