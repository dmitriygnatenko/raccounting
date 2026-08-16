package port

import (
	"context"
	"time"

	"raccounting/internal/domain/entity"
)

// SessionRepository persists login sessions.
type SessionRepository interface {
	Create(ctx context.Context, session entity.Session) error
	// FindByToken returns an error (not necessarily *domainerror.NotFoundError) whenever the token
	// doesn't map to a live session — callers treat any error here as "not authenticated" without
	// inspecting it further.
	FindByToken(ctx context.Context, token string) (entity.Session, error)
	Delete(ctx context.Context, token string) error
	// DeleteExpired removes every session whose expiry is before now.
	DeleteExpired(ctx context.Context, now time.Time) (int64, error)
}
