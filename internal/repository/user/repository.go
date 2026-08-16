// Package user implements port.UserRepository on top of the users table: it converts row models
// into domain entities and turns the storage layer's raw errors into domain errors — a missing row
// into *domainerror.NotFoundError, a username collision into *domainerror.ConflictError.
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

// conflictMessage is what both write paths report when a username is already taken.
const conflictMessage = "A user with this username is already registered"

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
	GetUserSettings(ctx context.Context, userID uint64) (model.UserSettings, error)

	// UpdateUserSettings overwrites userID's settings JSON blob.
	UpdateUserSettings(ctx context.Context, userID uint64, settings model.UserSettings) error

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

// FindByUsername looks up a user by their (already-normalized) username.
func (r *Repository) FindByUsername(ctx context.Context, username string) (entity.User, error) {
	m, err := r.storage.FindUserByUsername(ctx, username)
	if err != nil {
		return entity.User{}, notFound(err)
	}

	return m.ToEntity(), nil
}

// FindByID looks up a user by id.
func (r *Repository) FindByID(ctx context.Context, id uint64) (entity.User, error) {
	m, err := r.storage.FindUserByID(ctx, id)
	if err != nil {
		return entity.User{}, notFound(err)
	}

	return m.ToEntity(), nil
}

// Create inserts a new user and returns its id. A username collision is reported as a
// *domainerror.ConflictError.
func (r *Repository) Create(ctx context.Context, req port.UserCreateRequest) (uint64, error) {
	id, err := r.storage.CreateUser(ctx, req.Username, req.PasswordHash)
	if err != nil {
		return 0, conflict(err)
	}

	return id, nil
}

// UpdateUsername renames a user's login username. A collision with an existing username is reported
// as a *domainerror.ConflictError.
func (r *Repository) UpdateUsername(ctx context.Context, id uint64, username string) error {
	return conflict(r.storage.UpdateUsername(ctx, id, username))
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

// UpdateSettings overwrites a user's saved UI language.
func (r *Repository) UpdateSettings(ctx context.Context, id uint64, language string) error {
	return r.storage.UpdateUserSettings(ctx, id, model.UserSettings{Language: language})
}

// Count returns the total number of users — used to decide whether to auto-provision on login.
func (r *Repository) Count(ctx context.Context) (int, error) {
	return r.storage.CountUsers(ctx)
}

// notFound translates "no such row" into the domain's NotFoundError, passing any other error
// through.
func notFound(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return &domainerror.NotFoundError{Message: "User not found"}
	}

	return err
}

// conflict translates a unique-constraint violation into the domain's ConflictError, passing any
// other error (including nil) through.
func conflict(err error) error {
	if errors.Is(err, storageError.UniqueViolationError) {
		return &domainerror.ConflictError{Message: conflictMessage}
	}

	return err
}
