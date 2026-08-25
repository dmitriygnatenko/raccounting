package port

//go:generate go tool mockgen -source=user_repository.go -destination=mocks/user_repository_mock.go -package=mocks

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
	// FindByUsername returns a *domainerror.NotFoundError if no user with this (already-normalized)
	// username exists.
	FindByUsername(ctx context.Context, username string) (entity.User, error)
	// FindByID returns a *domainerror.NotFoundError if no user with this id exists.
	FindByID(ctx context.Context, id uint64) (entity.User, error)
	// Create returns a *domainerror.ConflictError if the username is already taken.
	Create(ctx context.Context, req UserCreateRequest) (id uint64, err error)
	// UpdateUsername returns a *domainerror.ConflictError if the new username is already taken.
	UpdateUsername(ctx context.Context, id uint64, username string) error
	UpdatePasswordHash(ctx context.Context, id uint64, hash string) error
	GetSettings(ctx context.Context, id uint64) (entity.UserSettings, error)
	// UpdateSettings overwrites a user's saved UI settings (language, theme).
	UpdateSettings(ctx context.Context, id uint64, settings entity.UserSettings) error
	// Count returns the total number of users — used to decide whether to auto-provision the demo
	// user.
	Count(ctx context.Context) (int, error)
}
