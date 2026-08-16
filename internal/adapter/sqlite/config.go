package sqlite

import (
	"fmt"
	"time"
)

// Config collects the SQLite connection parameters.
type Config struct {
	// Path is the on-disk path of the database file, created automatically (along with its parent
	// directory) on first run.
	Path string

	// ConnMaxLifetime is the only pool setting that applies to SQLite — MaxOpenConns is always 1
	// regardless of configuration (see Open).
	ConnMaxLifetime time.Duration
}

// dsn returns the connection string for the database file.
//
// _pragma=foreign_keys(1) turns on FK enforcement — off by default in SQLite, and must be set on
// every connection, hence via DSN rather than a one-off Exec after Open (the pool can open new
// connections at any time).
func (c Config) dsn() string {
	return fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)", c.Path)
}
