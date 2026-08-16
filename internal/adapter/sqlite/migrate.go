package sqlite

import (
	"context"
	"embed"
	"fmt"
	"io/fs"

	"github.com/pressly/goose/v3"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

// Migrate applies any pending migrations from migrations/ (goose format: -- +goose Up/Down),
// tracked in the goose_db_version table so each one runs at most once.
func Migrate(ctx context.Context, db *Storage) error {
	migrations, err := fs.Sub(migrationFiles, "migrations")
	if err != nil {
		return fmt.Errorf("reading embedded migrations: %w", err)
	}

	provider, err := goose.NewProvider(goose.DialectSQLite3, db.DB, migrations)
	if err != nil {
		return fmt.Errorf("creating migration provider: %w", err)
	}

	if _, err = provider.Up(ctx); err != nil {
		return fmt.Errorf("running migrations: %w", err)
	}

	return nil
}
