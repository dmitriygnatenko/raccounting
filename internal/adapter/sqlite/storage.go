// Package sqlite is the SQLite driver adapter: it implements every operation the repositories in
// internal/repository ask for, in SQLite's own dialect ("?" placeholders, LastInsertId for generated
// ids). The queries live next to this file, one file per table group (user.go, session.go,
// account.go, category.go, currency.go, transaction.go, transfer.go, budget.go,
// settings.go).
package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"modernc.org/sqlite"

	storageError "raccounting/internal/storage/error"
)

// Storage wraps the connection pool the queries run against. It satisfies each repository's own
// Storage interface; internal/app is where a single one of these is handed to all of them.
type Storage struct {
	*sql.DB
}

// wrapUnique normalizes a UNIQUE constraint violation into storageError.UniqueViolationError, so a
// repository can recognize it without knowing anything about SQLite. Any other error (including nil)
// passes through.
//
// Matching is done on the error message rather than sqlite.Error.Code(): SQLite's extended result
// code for a UNIQUE-style violation depends on which kind of constraint caught it (a plain UNIQUE
// index reports SQLITE_CONSTRAINT_UNIQUE/2067, but a composite PRIMARY KEY collision — as used by
// the currencies table — reports SQLITE_CONSTRAINT_PRIMARYKEY/1555 instead). Both, and every other
// row-uniqueness violation, share the same "UNIQUE constraint failed" message text, which is the
// stable thing to match on.
func wrapUnique(err error) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && strings.Contains(sqliteErr.Error(), "UNIQUE constraint failed") {
		return fmt.Errorf("%w: %w", storageError.UniqueViolationError, err)
	}

	return err
}

// wrapForeignKey normalizes a FOREIGN KEY constraint violation into
// storageError.ForeignKeyViolationError, so a repository can recognize it without knowing anything
// about SQLite. Any other error (including nil) passes through.
//
// Matching is done on the error message rather than sqlite.Error.Code(): an immediate FK violation
// (e.g. inserting a row whose parent doesn't exist) reports SQLITE_CONSTRAINT_FOREIGNKEY/787, but an
// ON DELETE RESTRICT violation — as used throughout this schema — reports
// SQLITE_CONSTRAINT_TRIGGER/1811 instead (RESTRICT is enforced through SQLite's internal trigger
// machinery). Both share the same "FOREIGN KEY constraint failed" message text, which is the stable
// thing to match on.
func wrapForeignKey(err error) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && strings.Contains(sqliteErr.Error(), "FOREIGN KEY constraint failed") {
		return fmt.Errorf("%w: %w", storageError.ForeignKeyViolationError, err)
	}

	return err
}

// wrapInsufficientBalance normalizes a write that took the accounts.balance CHECK (balance >= 0)
// constraint negative into storageError.InsufficientBalanceError, so a repository can recognize it
// without knowing anything about SQLite. SQLite has no unsigned integer type, so this CHECK is what
// stands in for MySQL's UNSIGNED column.
//
// The match is on "CHECK constraint failed: balance" specifically — SQLite's message includes the
// failing expression verbatim (e.g. "CHECK constraint failed: balance >= 0"), and this schema has
// other CHECK constraints too (categories.type, transactions.type/status, accounts.type), which a
// bare "CHECK constraint failed" match would misclassify as an insufficient balance. Any other error
// (including nil) passes through.
func wrapInsufficientBalance(err error) error {
	var sqliteErr *sqlite.Error
	if errors.As(err, &sqliteErr) && strings.Contains(sqliteErr.Error(), "CHECK constraint failed: balance") {
		return fmt.Errorf("%w: %w", storageError.InsufficientBalanceError, err)
	}

	return err
}

// insertReturningID runs an INSERT and returns the new row's id via LastInsertId.
func (s *Storage) insertReturningID(
	ctx context.Context, query string, args ...any,
) (uint64, error) {
	res, err := s.DB.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}

	id, err := res.LastInsertId()

	return uint64(id), err
}

// affected reports whether a statement touched any row, which is how the storage layer answers
// "found?" for updates and deletes.
func affected(res sql.Result) (bool, error) {
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}

	return n > 0, nil
}

// EnsureDatabase creates the parent directory of the database file if it doesn't exist yet — the
// driver creates the file itself on first connection.
func EnsureDatabase(cfg Config) error {
	if dir := filepath.Dir(cfg.Path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("creating directory %q for the SQLite file: %w", dir, err)
		}
	}

	return nil
}

// Open opens the connection pool for the database file and wraps it in Storage.
func Open(cfg Config) (*Storage, error) {
	raw, err := sql.Open("sqlite", cfg.dsn())
	if err != nil {
		return nil, fmt.Errorf("opening the database: %w", err)
	}

	raw.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	// SQLite doesn't tolerate concurrent writers — multiple simultaneous connections just produce
	// "database is locked" under load.
	raw.SetMaxOpenConns(1)

	if err := raw.Ping(); err != nil {
		return nil, fmt.Errorf("checking the database connection: %w", err)
	}

	return &Storage{DB: raw}, nil
}
