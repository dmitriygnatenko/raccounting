// Package user implements port.UserRepository on top of the users table: it converts row models
// into domain entities and turns the storage layer's raw errors into domain errors — a missing row
// into *domainerror.NotFoundError, a username collision into *domainerror.ConflictError. Both are
// message-less; the use case supplies the text.
package user

import (
	"context"
	"database/sql"
	"errors"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

//go:generate go tool mockgen -source=repository.go -destination=mocks/storage_mock.go -package=mocks

// Storage is the slice of the mysql adapter this repository uses — the users table and nothing
// else.
type Storage interface {
	// FindUserByUsername returns sql.ErrNoRows when no user has this (already-normalized) username.
	FindUserByUsername(ctx context.Context, username string) (model.User, error)
	// FindUserByID returns sql.ErrNoRows when no user has this id.
	FindUserByID(ctx context.Context, id uint64) (model.User, error)
	// CreateUser inserts a user row and returns its new id. A taken username comes back wrapped in
	// storageError.UniqueViolationError.
	CreateUser(ctx context.Context, username, passwordHash string) (id uint64, err error)
	// UpdateUsername renames a user. A taken username comes back wrapped in
	// storageError.UniqueViolationError.
	UpdateUsername(ctx context.Context, id uint64, username string) error
	UpdateUserPasswordHash(ctx context.Context, id uint64, hash string) error
	// GetUserSettings returns userID's settings JSON blob, zero-valued if the column is still NULL.
	GetUserSettings(ctx context.Context, id uint64) (model.UserSettings, error)
	// UpdateUserSettings overwrites userID's settings JSON blob.
	UpdateUserSettings(ctx context.Context, id uint64, settings model.UserSettings) error
	CountUsers(ctx context.Context) (int, error)
}

// Repository implements port.UserRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// FindByUsername looks up a user by their (already-normalized) username. An unknown username is
// reported as a message-less *domainerror.NotFoundError — the use case supplies the message.
func (r *Repository) FindByUsername(ctx context.Context, username string) (entity.User, error) {
	m, err := r.storage.FindUserByUsername(ctx, username)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.User{}, &domainerror.NotFoundError{}
		}

		return entity.User{}, err
	}

	return m.ToEntity(), nil
}

// FindByID looks up a user by id. An unknown id is reported as a message-less
// *domainerror.NotFoundError — the use case supplies the message.
func (r *Repository) FindByID(ctx context.Context, id uint64) (entity.User, error) {
	m, err := r.storage.FindUserByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.User{}, &domainerror.NotFoundError{}
		}

		return entity.User{}, err
	}

	return m.ToEntity(), nil
}

// Create inserts a new user and returns its id. A username collision is reported as a message-less
// *domainerror.ConflictError — the use case supplies the message.
func (r *Repository) Create(ctx context.Context, req port.UserCreateRequest) (uint64, error) {
	id, err := r.storage.CreateUser(ctx, req.Username, req.PasswordHash)
	if err != nil {
		if errors.Is(err, storageError.UniqueViolationError) {
			return 0, &domainerror.ConflictError{}
		}

		return 0, err
	}

	return id, nil
}

// UpdateUsername renames a user's login username. A collision with an existing username is reported
// as a message-less *domainerror.ConflictError — the use case supplies the message.
func (r *Repository) UpdateUsername(ctx context.Context, id uint64, username string) error {
	if err := r.storage.UpdateUsername(ctx, id, username); err != nil {
		if errors.Is(err, storageError.UniqueViolationError) {
			return &domainerror.ConflictError{}
		}

		return err
	}

	return nil
}

// UpdatePasswordHash overwrites a user's stored password hash.
func (r *Repository) UpdatePasswordHash(ctx context.Context, id uint64, hash string) error {
	return r.storage.UpdateUserPasswordHash(ctx, id, hash)
}

// GetSettings returns a user's saved settings.
func (r *Repository) GetSettings(ctx context.Context, id uint64) (entity.UserSettings, error) {
	m, err := r.storage.GetUserSettings(ctx, id)
	if err != nil {
		return entity.UserSettings{}, err
	}

	return m.ToEntity(), nil
}

// UpdateSettings overwrites a user's saved UI settings (language, theme).
func (r *Repository) UpdateSettings(ctx context.Context, id uint64, settings entity.UserSettings) error {
	return r.storage.UpdateUserSettings(ctx, id, model.UserSettings{
		Language: settings.Language,
		Theme:    settings.Theme,
	})
}

// Count returns the total number of users — used to decide whether to auto-provision on login.
func (r *Repository) Count(ctx context.Context) (int, error) {
	return r.storage.CountUsers(ctx)
}
