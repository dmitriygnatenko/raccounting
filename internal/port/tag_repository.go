package port

//go:generate go tool mockgen -source=tag_repository.go -destination=mocks/tag_repository_mock.go -package=mocks

import (
	"context"

	"raccounting/internal/domain/entity"
)

// TagCreateRequest bundles the TagRepository.Create parameters.
type TagCreateRequest struct {
	Name  string
	Color string
}

// TagUpdateRequest bundles the TagRepository.Update parameters.
type TagUpdateRequest struct {
	ID    uint64
	Name  string
	Color string
}

// TagRepository persists Tags.
type TagRepository interface {
	List(ctx context.Context) ([]entity.Tag, error)
	// FindByIDs returns every tag among ids that exists — used to validate a transaction's tags
	// before saving it (the caller compares len(result) against the number of distinct ids given).
	FindByIDs(ctx context.Context, ids []uint64) ([]entity.Tag, error)
	Create(ctx context.Context, req TagCreateRequest) (entity.Tag, error)
	// Update returns a *domainerror.NotFoundError if no tag with this id exists.
	Update(ctx context.Context, req TagUpdateRequest) (entity.Tag, error)
	// Delete returns a *domainerror.NotFoundError if no tag with this id exists. Deleting a tag
	// always succeeds otherwise — it just detaches from any transaction that had it (see the
	// transaction_tags ON DELETE CASCADE in the adapter migrations).
	Delete(ctx context.Context, id uint64) error
}
