// Package session implements port.SessionRepository on top of the sessions table.
package session

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/storage/model"
)

// Storage is the slice of the mysql adapter this repository uses — the sessions table and nothing
// else.
type Storage interface {
	CreateSession(ctx context.Context, session model.Session) error
	// FindSessionByToken returns sql.ErrNoRows when no session has this token.
	FindSessionByToken(ctx context.Context, token string) (model.Session, error)
	DeleteSession(ctx context.Context, token string) error
	DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error)
}

// Repository implements port.SessionRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// Create inserts a new session row.
func (r *Repository) Create(ctx context.Context, session entity.Session) error {
	return r.storage.CreateSession(ctx, model.Session{
		Token:     session.Token,
		UserID:    session.UserID,
		ExpiresAt: session.ExpiresAt,
	})
}

// FindByToken looks up a session by its token. An unknown token is reported as a
// *domainerror.NotFoundError.
func (r *Repository) FindByToken(ctx context.Context, token string) (entity.Session, error) {
	m, err := r.storage.FindSessionByToken(ctx, token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Session{}, &domainerror.NotFoundError{Message: "Session not found"}
		}

		return entity.Session{}, err
	}

	return m.ToEntity(), nil
}

// Delete removes a session by its token.
func (r *Repository) Delete(ctx context.Context, token string) error {
	return r.storage.DeleteSession(ctx, token)
}

// DeleteExpired removes every session that expired before now.
func (r *Repository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	return r.storage.DeleteExpiredSessions(ctx, now)
}
