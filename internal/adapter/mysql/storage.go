// Package mysql is the MySQL/MariaDB driver adapter: it implements every operation the repositories
// in internal/repository ask for, in MySQL's own dialect ("?" placeholders, LastInsertId for
// generated ids). The queries live next to this file, one file per table group (user.go, session.go,
// account.go, category.go, currency.go, transaction.go, transfer.go, budget.go,
// settings.go).
package mysql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"regexp"

	mysqldriver "github.com/go-sql-driver/mysql"

	storageError "raccounting/internal/storage/error"
)

var identifierRE = regexp.MustCompile(`^[A-Za-z0-9_]+$`)

// Storage wraps the connection pool the queries run against. It satisfies each repository's own
// Storage interface; internal/app is where a single one of these is handed to all of them.
type Storage struct {
	*sql.DB
}

// MySQL error numbers this adapter translates into the storage package's driver-agnostic sentinels.
const (
	errDuplicateEntry  = 1062 // ER_DUP_ENTRY — a UNIQUE constraint was violated.
	errRowIsReferenced = 1451 // ER_ROW_IS_REFERENCED_2 — a FOREIGN KEY constraint blocked a DELETE/UPDATE.

	// A write took the unsigned accounts.balance column negative — which of these two fires depends
	// on how the value got there: assigning a negative literal directly (SET balance = -5, as
	// account.go's CreateAccount/UpdateAccount do) raises errWarnDataOutOfRange, while an arithmetic
	// expression underflowing (SET balance = balance + -2000, as every balance UPDATE in
	// transaction.go/transfer.go does) raises errDataOutOfRange instead. Both are handled.
	errWarnDataOutOfRange = 1264 // ER_WARN_DATA_OUT_OF_RANGE
	errDataOutOfRange     = 1690 // ER_DATA_OUT_OF_RANGE
)

// wrapUnique normalizes a UNIQUE constraint violation into storageError.UniqueViolationError, so a
// repository can recognize it without knowing anything about MySQL. Any other error (including nil)
// passes through.
func wrapUnique(err error) error {
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == errDuplicateEntry {
		return fmt.Errorf("%w: %w", storageError.UniqueViolationError, err)
	}

	return err
}

// wrapForeignKey normalizes a FOREIGN KEY constraint violation into
// storageError.ForeignKeyViolationError, so a repository can recognize it without knowing anything
// about MySQL. Any other error (including nil) passes through.
func wrapForeignKey(err error) error {
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && mysqlErr.Number == errRowIsReferenced {
		return fmt.Errorf("%w: %w", storageError.ForeignKeyViolationError, err)
	}

	return err
}

// wrapInsufficientBalance normalizes a write that took the unsigned accounts.balance column
// negative into storageError.InsufficientBalanceError, so a repository can recognize it without
// knowing anything about MySQL. Any other error (including nil) passes through.
func wrapInsufficientBalance(err error) error {
	var mysqlErr *mysqldriver.MySQLError
	if errors.As(err, &mysqlErr) && (mysqlErr.Number == errWarnDataOutOfRange || mysqlErr.Number == errDataOutOfRange) {
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

// EnsureDatabase creates the target database if it doesn't already exist. This needs CREATE
// DATABASE privileges; without them, just make sure the database exists ahead of time and this call
// will fail harmlessly for the caller to log and ignore.
func EnsureDatabase(cfg Config) error {
	if !identifierRE.MatchString(cfg.Name) {
		return fmt.Errorf("invalid database name: %q", cfg.Name)
	}

	root, err := sql.Open("mysql", cfg.serverDSN())
	if err != nil {
		return fmt.Errorf("connecting to the DB server: %w", err)
	}
	defer root.Close()

	if err := root.Ping(); err != nil {
		return fmt.Errorf("reaching the DB server: %w", err)
	}

	stmt := fmt.Sprintf(
		"CREATE DATABASE IF NOT EXISTS `%s` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci",
		cfg.Name,
	)
	if _, err := root.Exec(stmt); err != nil {
		return fmt.Errorf("creating database %q: %w", cfg.Name, err)
	}

	return nil
}

// Open opens the connection pool for the target database and wraps it in Storage.
func Open(cfg Config) (*Storage, error) {
	raw, err := sql.Open("mysql", cfg.dsn())
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
