package port

import (
	"context"

	"raccounting/internal/domain/entity"
)

// UserCreateRequest bundles the UserRepository.Create parameters that ride along with the context.
type UserCreateRequest struct {
	Username     string
	PasswordHash string
}

// UserRepository persists the single user account. Create returns a *domainerror.ConflictError if a
// user with the given username already exists — in practice this can only happen for the second row
// ever inserted, since login only ever auto-provisions once the table is empty.
type UserRepository interface {
	FindByUsername(ctx context.Context, username string) (entity.User, error)
	FindByID(ctx context.Context, id uint64) (entity.User, error)
	Create(ctx context.Context, req UserCreateRequest) (id uint64, err error)
	UpdateUsername(ctx context.Context, id uint64, username string) error
	UpdatePasswordHash(ctx context.Context, id uint64, hash string) error
	GetSettings(ctx context.Context, id uint64) (entity.UserSettings, error)
	UpdateSettings(ctx context.Context, id uint64, language string) error
	Count(ctx context.Context) (int, error)
}
