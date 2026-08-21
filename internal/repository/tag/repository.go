// Package tag implements port.TagRepository on top of the tags table.
package tag

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

// Storage is the slice of the DB adapter this repository uses — the tags table and nothing else.
type Storage interface {
	ListTags(ctx context.Context) ([]model.Tag, error)
	// FindTagsByIDs returns every tag among ids that exists.
	FindTagsByIDs(ctx context.Context, ids []uint64) ([]model.Tag, error)
	// CreateTag inserts a tag row and returns its new id. A name collision comes back wrapped in
	// storageError.UniqueViolationError.
	CreateTag(ctx context.Context, req model.TagCreateRequest) (id uint64, err error)
	// UpdateTag changes name/color, returning the full updated row. found is false if no tag with
	// this id exists. A name collision comes back wrapped in storageError.UniqueViolationError.
	UpdateTag(
		ctx context.Context,
		req model.TagUpdateRequest,
	) (row model.Tag, found bool, err error)
	// DeleteTag removes a tag row. found is false if no tag with this id existed.
	DeleteTag(ctx context.Context, id uint64) (found bool, err error)
}

// Repository implements port.TagRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every tag.
func (r *Repository) List(ctx context.Context) ([]entity.Tag, error) {
	rows, err := r.storage.ListTags(ctx)
	if err != nil {
		return nil, err
	}

	tags := make([]entity.Tag, len(rows))
	for i, row := range rows {
		tags[i] = row.ToEntity()
	}

	return tags, nil
}

// FindByIDs returns every tag among ids that exists.
func (r *Repository) FindByIDs(ctx context.Context, ids []uint64) ([]entity.Tag, error) {
	rows, err := r.storage.FindTagsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	tags := make([]entity.Tag, len(rows))
	for i, row := range rows {
		tags[i] = row.ToEntity()
	}

	return tags, nil
}

// Create inserts a new tag and returns it. A name already in use is reported as a message-less
// *domainerror.ConflictError — the use case supplies the message.
func (r *Repository) Create(
	ctx context.Context, req port.TagCreateRequest,
) (entity.Tag, error) {
	id, err := r.storage.CreateTag(ctx, model.TagCreateRequest{
		Name:  req.Name,
		Color: req.Color,
	})
	if err != nil {
		if errors.Is(err, storageError.UniqueViolationError) {
			return entity.Tag{}, &domainerror.ConflictError{}
		}

		return entity.Tag{}, err
	}

	return entity.Tag{
		ID:    id,
		Name:  req.Name,
		Color: req.Color,
	}, nil
}

// Update changes an existing tag's name/color. An unknown id is reported as a message-less
// *domainerror.NotFoundError, and a name already in use as a message-less
// *domainerror.ConflictError — the use case supplies the message either way.
func (r *Repository) Update(
	ctx context.Context,
	req port.TagUpdateRequest,
) (entity.Tag, error) {
	row, found, err := r.storage.UpdateTag(ctx, model.TagUpdateRequest{
		ID:    req.ID,
		Name:  req.Name,
		Color: req.Color,
	})
	if err != nil {
		if errors.Is(err, storageError.UniqueViolationError) {
			return entity.Tag{}, &domainerror.ConflictError{}
		}

		return entity.Tag{}, err
	}

	if !found {
		return entity.Tag{}, &domainerror.NotFoundError{}
	}

	return row.ToEntity(), nil
}

// Delete removes a tag, reporting a *domainerror.NotFoundError if it doesn't exist. Both are
// message-less — the use case supplies the message.
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	found, err := r.storage.DeleteTag(ctx, id)
	if err != nil {
		return err
	}

	if !found {
		return &domainerror.NotFoundError{}
	}

	return nil
}
