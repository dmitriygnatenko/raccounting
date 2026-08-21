# Raccounting

A self-hosted personal finance tracker: a single Go binary with an embedded vanilla-JS frontend,
backed by MySQL, PostgreSQL or SQLite.

## Features

- **Accounts** — multiple accounts with independent balances and currencies.
- **Transactions & transfers** — income/expense entries with categories and tags, plus transfers
  between accounts.
- **Budgets** — per-category monthly budgets with spend tracking.
- **Multi-currency** — configurable currencies and a base currency for aggregate totals.
- **Dashboard & reports** — income/expense breakdowns, spending by category, balance history.
- **Backup & restore** — export the entire database as one JSON snapshot and import it back.
- **Auth** — cookie-based sessions, single demo user seeded on first run.
- **i18n** — English, Russian, Spanish, German, French.
- **One binary** — the frontend is embedded via `go:embed`; there's nothing to build or serve
  separately in production.

## Requirements

- Go 1.26+
- One of: MySQL, PostgreSQL, or SQLite (no server needed for SQLite)
- Docker, if you want to run MariaDB via the bundled `docker-compose.yml`

## Quick start

```bash
cp .env.example .env
docker compose up -d   # starts MariaDB for local development
go run ./cmd/raccounting
```

The app listens on `http://localhost:8080` (see `PORT`). Migrations run automatically on startup,
and a demo user is seeded the first time the `users` table is empty (`DEMO_USERNAME` /
`DEMO_PASSWORD`, default `user` / `pass`).

To run against SQLite instead — no Docker needed:

```bash
DB_DRIVER=sqlite DB_SQLITE_PATH=./data/raccounting.db go run ./cmd/raccounting
```

## Configuration

All configuration comes from environment variables, loaded from `.env` in development (see
`.env.example` for the full, commented list). A value that's set but malformed fails at startup
rather than falling back silently.

| Variable | Default | Description |
|---|---|---|
| `DB_DRIVER` | `mysql` | `mysql`, `postgres`, or `sqlite` |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | — | Connection settings for mysql/postgres |
| `DB_SQLITE_PATH` | — | Database file path, required for sqlite |
| `DB_MAX_OPEN_CONNS` / `DB_MAX_IDLE_CONNS` / `DB_CONN_MAX_LIFETIME` / `DB_CONN_TIMEOUT` | — | Connection pool tuning (ignored for sqlite) |
| `PORT` | `8080` | HTTP listen port |
| `COOKIE_SECURE` | `false` | Send the session cookie over HTTPS only — enable in production |
| `DEMO_USERNAME` / `DEMO_PASSWORD` | `user` / `pass` | Seeded demo user credentials (first run only) |
| `LOG_CONSOLE_LEVEL` | `warn` | Console log level: `debug`, `info`, `warn`, `error` |
| `LOG_FILE_PATH` / `LOG_FILE_LEVEL` | — | Optional JSON file log, off until a path is set |

## Development

```bash
make run            # go run ./cmd/raccounting
make build           # build a Linux amd64 binary into build/app/raccounting
make test            # go test ./...
make fmt vet         # format / static analysis
make docker-up       # start MariaDB
make docker-down     # stop MariaDB
```

Run `make help` for the full list of targets.

### Docker image

```bash
make docker-build                     # builds raccounting:latest
docker run -p 8080:8080 --env-file .env raccounting:latest
```

The image is a multi-stage build (`Dockerfile`): a `golang:1.26-alpine` stage compiles a static,
`CGO_ENABLED=0` binary, which is then copied into a minimal `alpine` runtime image alongside
`ca-certificates`. Override the image name/tag with `IMAGE_NAME` / `IMAGE_TAG`.

## Architecture

The backend follows a hexagonal layout under `internal/`:

- `domain/` — entities and use cases, the core business logic, with no framework dependencies.
- `port/` — interfaces the domain depends on (repositories, password hashing, token generation).
- `adapter/` — driving/driven adapters: the HTTP API (`adapter/http`) and the database drivers
  (`adapter/mysql`, `adapter/postgres`, `adapter/sqlite`), each implementing the same ports.
- `repository/` — storage-agnostic repository implementations shared across drivers.
- `app/` — wiring: builds the storage, repositories, use cases and HTTP server from config.

The frontend (`web/`) is a dependency-free vanilla-JS SPA, embedded into the binary at build time.

## License

MIT — see [LICENSE](LICENSE).
