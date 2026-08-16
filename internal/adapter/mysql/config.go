package mysql

import (
	"net"
	"time"

	mysqldriver "github.com/go-sql-driver/mysql"
)

// Config collects the MySQL/MariaDB connection parameters.
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string

	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnTimeout     time.Duration
}

// dsnConfig builds the connection settings shared by dsn/serverDSN, via the driver's own
// Config/FormatDSN rather than hand-built string concatenation — that way User/Password are escaped
// correctly even if they contain characters like '@' or ':' that would otherwise break the DSN's
// syntax.
func (c Config) dsnConfig() mysqldriver.Config {
	cfg := mysqldriver.NewConfig()
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(c.Host, c.Port)
	cfg.User = c.User
	cfg.Passwd = c.Password
	cfg.ParseTime = true
	cfg.Loc = time.UTC
	cfg.Timeout = c.ConnTimeout
	cfg.Params = map[string]string{
		"charset":   "utf8mb4",
		"time_zone": "'+00:00'",
	}

	return *cfg
}

// dsn returns the connection string for the target database.
func (c Config) dsn() string {
	cfg := c.dsnConfig()
	cfg.DBName = c.Name

	return cfg.FormatDSN()
}

// serverDSN connects to the DB server without selecting a database — used only to create the
// database on first run.
func (c Config) serverDSN() string {
	cfg := c.dsnConfig()

	return cfg.FormatDSN()
}
