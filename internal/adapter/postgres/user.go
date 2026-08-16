package postgres

import (
	"context"
	"database/sql"

	"raccounting/internal/storage/model"
)

// FindUserByUsername looks up a user by their username.
func (s *Storage) FindUserByUsername(ctx context.Context, username string) (model.User, error) {
	return scanUser(s.DB.QueryRowContext(ctx,
		`SELECT id, username, password_hash, settings FROM users WHERE username = $1`, username,
	))
}

// FindUserByID looks up a user by id.
func (s *Storage) FindUserByID(ctx context.Context, id uint64) (model.User, error) {
	return scanUser(s.DB.QueryRowContext(ctx,
		`SELECT id, username, password_hash, settings FROM users WHERE id = $1`, id,
	))
}

// CreateUser inserts a user row and returns its new id.
func (s *Storage) CreateUser(ctx context.Context, username, passwordHash string) (uint64, error) {
	id, err := s.insertReturningID(ctx,
		`INSERT INTO users (username, password_hash) VALUES ($1, $2) RETURNING id`,
		username, passwordHash,
	)
	if err != nil {
		return 0, wrapUnique(err)
	}

	return id, nil
}

// UpdateUsername renames a user.
func (s *Storage) UpdateUsername(ctx context.Context, id uint64, username string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE users SET username = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, username, id,
	)

	return wrapUnique(err)
}

// UpdateUserPasswordHash overwrites a user's stored password hash.
func (s *Storage) UpdateUserPasswordHash(ctx context.Context, id uint64, hash string) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE users SET password_hash = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, hash, id,
	)

	return err
}

// CountUsers returns the total number of users.
func (s *Storage) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.DB.QueryRowContext(ctx, `SELECT COUNT(*) FROM users`).Scan(&n)

	return n, err
}

// GetUserSettings returns userID's settings JSON blob, zero-valued if the column is still NULL.
func (s *Storage) GetUserSettings(ctx context.Context, userID uint64) (model.UserSettings, error) {
	var settings model.UserSettings

	err := s.DB.QueryRowContext(ctx, `SELECT settings FROM users WHERE id = $1`, userID).Scan(&settings)

	return settings, err
}

// UpdateUserSettings overwrites userID's settings JSON blob.
func (s *Storage) UpdateUserSettings(ctx context.Context, userID uint64, settings model.UserSettings) error {
	_, err := s.DB.ExecContext(ctx,
		`UPDATE users SET settings = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`, settings, userID,
	)

	return err
}

// scanUser reads the user column list, in the order every user query above selects it.
func scanUser(row *sql.Row) (model.User, error) {
	var m model.User
	if err := row.Scan(&m.ID, &m.Username, &m.PasswordHash, &m.Settings); err != nil {
		return model.User{}, err
	}

	return m, nil
}
