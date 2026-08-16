package postgres

import (
	"context"
	"time"

	"raccounting/internal/storage/model"
)

// CreateSession inserts a session row.
func (s *Storage) CreateSession(ctx context.Context, session model.Session) error {
	_, err := s.DB.ExecContext(ctx,
		`INSERT INTO sessions (token, user_id, expires_at) VALUES ($1, $2, $3)`,
		session.Token, session.UserID, session.ExpiresAt,
	)

	return err
}

// FindSessionByToken looks up a session by its token, returning sql.ErrNoRows when there's no match.
func (s *Storage) FindSessionByToken(ctx context.Context, token string) (model.Session, error) {
	m := model.Session{Token: token}

	err := s.DB.QueryRowContext(ctx,
		`SELECT user_id, expires_at FROM sessions WHERE token = $1`, token,
	).Scan(&m.UserID, &m.ExpiresAt)
	if err != nil {
		return model.Session{}, err
	}

	return m, nil
}

// DeleteSession removes a session by its token.
func (s *Storage) DeleteSession(ctx context.Context, token string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE token = $1`, token)

	return err
}

// DeleteExpiredSessions removes every session that expired before now.
func (s *Storage) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM sessions WHERE expires_at < $1`, now)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}
