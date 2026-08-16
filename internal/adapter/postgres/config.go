package postgres

import (
	"net"
	"net/url"
	"time"
)

// Config collects the Postgres connection parameters.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
}

// dsnFor builds a connection URL for the given database name via net/url — User/Password/dbName are
// percent-encoded automatically, so characters like '@', '/' or ':' in them can't break the
// connection string's syntax.
func (c Config) dsnFor(dbName string) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(c.User, c.Password),
		Host:   net.JoinHostPort(c.Host, c.Port),
		Path:   "/" + dbName,
	}
	q := u.Query()
	q.Set("sslmode", "disable")
	u.RawQuery = q.Encode()

	return u.String()
}

// dsn returns the connection string for the target database.
func (c Config) dsn() string {
	return c.dsnFor(c.Name)
}

// serverDSN connects to the DB server without selecting a database — used only to create the
// database on first run. Postgres requires connecting to some existing database to create another
// one from there; "postgres" is the conventional service DB.
func (c Config) serverDSN() string {
	return c.dsnFor("postgres")
}
