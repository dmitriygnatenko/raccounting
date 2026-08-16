// Package postgres is the Postgres driver adapter: it implements every operation the repositories in
// internal/repository ask for, in Postgres's own dialect ("$1, $2, ..." placeholders, RETURNING id
// for generated ids). The queries live next to this file, one file per table group (user.go,
// session.go, account.go, category.go, currency.go, transaction.go, transfer.go, budget.go,
// settings.go).
package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used below

	storageError "raccounting/internal/storage/error"
)

var identifierRE = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Storage wraps the connection pool the queries run against. It satisfies each repository's own
// Storage interface; internal/app is where a single one of these is handed to all of them.
type Storage struct {
	*sql.DB
}

// Postgres SQLSTATE codes this adapter translates into the storage package's driver-agnostic
// sentinels.
const (
	pgUniqueViolation     = "23505"
	pgForeignKeyViolation = "23503"
	pgCheckViolation      = "23514"
)

// pgBalanceCheckConstraint is the name Postgres auto-generates for accounts' unnamed
// CHECK (balance >= 0) constraint ("<table>_<column>_check") — this schema has other CHECK
// constraints too (categories.type, transactions.type/status, accounts.type), all sharing SQLSTATE
// 23514, so the constraint name is what disambiguates an insufficient-balance write from those.
const pgBalanceCheckConstraint = "accounts_balance_check"

// wrapUnique normalizes a UNIQUE constraint violation into storageError.UniqueViolationError, so a
// repository can recognize it without knowing anything about Postgres. Any other error (including
// nil) passes through.
func wrapUnique(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation {
		return fmt.Errorf("%w: %w", storageError.UniqueViolationError, err)
	}

	return err
}

// wrapForeignKey normalizes a FOREIGN KEY constraint violation into
// storageError.ForeignKeyViolationError, so a repository can recognize it without knowing anything
// about Postgres. Any other error (including nil) passes through.
func wrapForeignKey(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgForeignKeyViolation {
		return fmt.Errorf("%w: %w", storageError.ForeignKeyViolationError, err)
	}

	return err
}

// wrapInsufficientBalance normalizes a write that took the accounts.balance CHECK (balance >= 0)
// constraint negative into storageError.InsufficientBalanceError, so a repository can recognize it
// without knowing anything about Postgres. Postgres has no unsigned integer type, so this CHECK is
// what stands in for MySQL's UNSIGNED column. Any other error (including nil) passes through.
func wrapInsufficientBalance(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == pgCheckViolation && pgErr.ConstraintName == pgBalanceCheckConstraint {
		return fmt.Errorf("%w: %w", storageError.InsufficientBalanceError, err)
	}

	return err
}

// insertReturningID runs an INSERT with a RETURNING id clause — Postgres drivers don't support
// LastInsertId.
func (s *Storage) insertReturningID(
	ctx context.Context, query string, args ...any,
) (uint64, error) {
	var id uint64
	err := s.DB.QueryRowContext(ctx, query, args...).Scan(&id)

	return id, err
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

// EnsureDatabase creates the target database if it doesn't already exist. This needs CREATE
// DATABASE privileges; without them, just make sure the database exists ahead of time and this call
// will fail harmlessly for the caller to log and ignore.
func EnsureDatabase(cfg Config) error {
	if !identifierRE.MatchString(cfg.Name) {
		return fmt.Errorf("invalid database name: %q", cfg.Name)
	}

	root, err := sql.Open("pgx", cfg.serverDSN())
	if err != nil {
		return fmt.Errorf("connecting to the DB server: %w", err)
	}
	defer root.Close()

	if err := root.Ping(); err != nil {
		return fmt.Errorf("reaching the DB server: %w", err)
	}

	var exists bool

	err = root.QueryRow(`SELECT EXISTS (SELECT 1 FROM pg_database WHERE datname = $1)`, cfg.Name).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking whether database %q exists: %w", cfg.Name, err)
	}

	if exists {
		return nil
	}
	// CREATE DATABASE doesn't support IF NOT EXISTS or a parameterized name — but cfg.Name is already
	// validated by identifierRE above.
	if _, err := root.Exec(fmt.Sprintf(`CREATE DATABASE "%s"`, cfg.Name)); err != nil {
		return fmt.Errorf("creating database %q: %w", cfg.Name, err)
	}

	return nil
}

// Open opens the connection pool for the target database and wraps it in Storage.
func Open(cfg Config) (*Storage, error) {
	raw, err := sql.Open("pgx", cfg.dsn())
	if err != nil {
		return nil, fmt.Errorf("opening the database: %w", err)
	}

	raw.SetMaxOpenConns(cfg.MaxOpenConns)
	raw.SetMaxIdleConns(cfg.MaxIdleConns)
	raw.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	if err := raw.Ping(); err != nil {
		return nil, fmt.Errorf("checking the database connection: %w", err)
	}

	return &Storage{DB: raw}, nil
}
