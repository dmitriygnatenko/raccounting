package config

import (
	"fmt"
	"os"
	"time"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Supported values for DBConfig.Driver.
const (
	DriverMySQL    = "mysql"
	DriverPostgres = "postgres"
	DriverSQLite   = "sqlite"
)

// defaultDriver is used whenever DB_DRIVER isn't set, so existing MySQL/MariaDB deployments keep
// working unchanged.
const defaultDriver = DriverMySQL

// Connection pool defaults, used whenever the corresponding env var isn't set. They don't apply to
// SQLite's max-open-conns, which is always 1 regardless of configuration — SQLite doesn't tolerate
// concurrent writers.
const (
	defaultMaxOpenConns    = 10
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = 5 * time.Minute
	defaultConnTimeout     = 5 * time.Second
)

// DBConfig collects DB connection parameters. Some fields only apply to a subset of drivers:
// Host/Port/User/Password/Name — MySQL and Postgres only; SQLitePath — SQLite only; ConnTimeout —
// MySQL only (the dial timeout for the initial TCP connection). MaxOpenConns/MaxIdleConns/
// ConnMaxLifetime apply to every driver (SQLite ignores MaxOpenConns/MaxIdleConns — see
// internal/adapter/sqlite).
type DBConfig struct {
	Driver     string
	Host       string
	Port       string
	User       string
	Password   string
	Name       string
	SQLitePath string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnTimeout     time.Duration
}

// validate checks that every setting the chosen driver actually needs is present, plus the bounds
// on the already-parsed pool settings. Password is deliberately not required — plenty of local/dev
// setups run with an empty DB password. Host/Port/User/Name are only required for the networked
// drivers (mysql/postgres); SQLitePath is only required for sqlite — validation.When applies each
// rule conditionally on which driver is selected.
func (cfg DBConfig) validate() error {
	isSQLite := cfg.Driver == DriverSQLite

	return validation.ValidateStruct(&cfg,
		validation.Field(
			&cfg.Driver, validation.Required.Error("DB_DRIVER must not be empty"),
			validation.In(DriverMySQL, DriverPostgres, DriverSQLite).
				Error(fmt.Sprintf("invalid DB_DRIVER %q: must be %q, %q or %q",
					cfg.Driver, DriverMySQL, DriverPostgres, DriverSQLite),
				),
		),
		validation.Field(
			&cfg.Host,
			validation.When(!isSQLite, validation.Required.Error("DB_HOST must not be empty for "+cfg.Driver)),
		),
		validation.Field(
			&cfg.Port,
			validation.When(!isSQLite, validation.Required.Error("DB_PORT must not be empty for "+cfg.Driver)),
		),
		validation.Field(
			&cfg.User,
			validation.When(!isSQLite, validation.Required.Error("DB_USER must not be empty for "+cfg.Driver)),
		),
		validation.Field(
			&cfg.Name,
			validation.When(!isSQLite, validation.Required.Error("DB_NAME must not be empty for "+cfg.Driver)),
		),
		validation.Field(
			&cfg.SQLitePath,
			validation.When(isSQLite, validation.Required.Error("DB_SQLITE_PATH must not be empty for sqlite")),
		),
		validation.Field(&cfg.MaxOpenConns, validation.Min(1).Error("DB_MAX_OPEN_CONNS must be at least 1")),
		validation.Field(&cfg.MaxIdleConns, validation.Min(0).Error("DB_MAX_IDLE_CONNS must not be negative")),
		validation.Field(&cfg.ConnMaxLifetime,
			validation.Min(time.Duration(0)).Error("DB_CONN_MAX_LIFETIME must not be negative")),
		validation.Field(&cfg.ConnTimeout,
			validation.Min(time.Duration(0)).Error("DB_CONN_TIMEOUT must not be negative")),
	)
}

// LoadDB builds the database configuration from the environment and validates it before returning.
// Connection settings the chosen driver needs (host, credentials, ...) have no implicit defaults —
// they must be set explicitly, or LoadDB fails fast at startup with a clear message instead of
// silently picking a value the caller never asked for. Pool tuning is the exception: those are
// genuinely optional and fall back to sensible defaults when unset.
func LoadDB() (DBConfig, error) {
	maxOpenConns, err := intEnv("DB_MAX_OPEN_CONNS", defaultMaxOpenConns)
	if err != nil {
		return DBConfig{}, err
	}

	maxIdleConns, err := intEnv("DB_MAX_IDLE_CONNS", defaultMaxIdleConns)
	if err != nil {
		return DBConfig{}, err
	}

	connMaxLifetime, err := durationEnv("DB_CONN_MAX_LIFETIME", defaultConnMaxLifetime)
	if err != nil {
		return DBConfig{}, err
	}

	connTimeout, err := durationEnv("DB_CONN_TIMEOUT", defaultConnTimeout)
	if err != nil {
		return DBConfig{}, err
	}

	cfg := DBConfig{
		Driver: stringEnv("DB_DRIVER", defaultDriver),
		Host:   stringEnv("DB_HOST", ""),
		Port:   stringEnv("DB_PORT", ""),
		User:   stringEnv("DB_USER", ""),
		// Read raw, unlike every other field: a password may legitimately begin or end with a space, and
		// silently trimming one would turn a correct password into a failed connection.
		Password:   os.Getenv("DB_PASSWORD"),
		Name:       stringEnv("DB_NAME", ""),
		SQLitePath: stringEnv("DB_SQLITE_PATH", ""),

		MaxOpenConns:    maxOpenConns,
		MaxIdleConns:    maxIdleConns,
		ConnMaxLifetime: connMaxLifetime,
		ConnTimeout:     connTimeout,
	}
	if err = cfg.validate(); err != nil {
		return DBConfig{}, err
	}

	return cfg, nil
}
