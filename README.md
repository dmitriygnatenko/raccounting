# Raccounting

Raccounting is a self-hosted personal finance tracker: **accounts** (cash, cards, bank accounts,
savings, credit cards, debts, virtual), **transactions** (income/expense, categorized and tagged),
**transfers** between accounts (including cross-currency ones, at a rate you set), monthly
**per-category budgets**, and a dashboard/reports view over all of it. It's a single-user app —
there's no multi-tenant account model, just one signed-in user and their data.

The project is a single Go binary: a REST API backed by your choice of MySQL/MariaDB, PostgreSQL or
SQLite, with the entire frontend embedded inside it via `go:embed`. There is no frontend build step
— the UI is a dependency-free Vue 3 app loaded straight from a CDN — and no CORS setup to worry
about: one binary listens on one port and serves both the API and the UI.

## Highlights

- **Accounts with real balances** — cash, card, bank account, savings, credit card, debt or
  virtual, each denominated in one currency, with a balance kept in sync (atomically, in the same
  DB transaction) by every transaction, transfer and reversal touching it. A balance can never go
  negative: creating, editing or deleting a transaction/transfer that would push an account below
  zero is rejected with a 409 Conflict instead of silently corrupting the balance.
- **Transactions, categories and tags** — income/expense entries carry one category (income or
  expense, never both) and any number of freeform tags, independent of category, for cutting the
  numbers a different way (e.g. "reimbursable" across several categories).
- **Transfers** — move money between two accounts in one atomic operation, stored as two linked
  transaction legs (debit + credit) rather than one row, so each leg keeps its own account's
  currency; moving between accounts in different currencies records the rate applied.
- **Per-category monthly budgets** — plan how much to spend in a category for a given month
  (`YYYY-MM`) and track actual spend against it; the UI surfaces a notification when a category
  goes over.
- **Multi-currency** — an arbitrary set of currencies, one marked default; every other currency
  carries a rate relative to it so the dashboard can show one aggregate total.
- **Dashboard & reports** — total balance, income/expense breakdowns, spending by category, and a
  paginated, filterable transaction list (by date range, account, category, tag, type or free-text
  search), with running sums per currency for the filtered set, not just the visible page.
- **Backup & restore** — export every table (minus login credentials) as one versioned JSON
  snapshot, and import it back. Restore doesn't copy balances directly: it recreates each account
  at the balance it had *before* the backup's transactions, then replays every transaction/transfer
  oldest-first through the same balance-never-negative checks that built the original balance — so
  a restore can only succeed if the backup is internally consistent.
- **Real, cookie-based authentication** — bcrypt-hashed password, a random session token, an
  httpOnly `SameSite=Lax` cookie, and an hourly background sweep of expired sessions. There's no
  registration flow or per-user data isolation to reason about, since the whole app is one user's
  ledger; credentials (username/password) are changed separately from other settings and always
  re-confirm the current password.
- **Five interface languages** — Russian, English, Spanish, German and French. Russian is the
  source language the frontend is written in; the other four are exact-match translation tables
  (see [Internationalization](#internationalization)).
- **Three interchangeable database backends** — MySQL/MariaDB, PostgreSQL or SQLite — selected by
  one environment variable, each with its own adapter, schema and idiomatic dialect (parameter
  placeholders, `LAST_INSERT_ID` vs `RETURNING`, etc).
- **Fail-fast configuration** — every setting is validated at startup; a missing or malformed
  environment variable stops the process with a clear message instead of silently doing the wrong
  thing.

## Screenshots / how it works

Log in (or use the seeded demo account) and you land on the **Overview** dashboard: total balance
across accounts, income/expense summaries, and recent activity. **Accounts** lists every
cash/card/bank/savings/credit-card/debt/virtual account with its live balance, lets you archive
ones you no longer use, and is where transfers are made. **Transactions** is the full, filterable
ledger — search by date range, account, category, tag, type or free text, with a running total per
currency for whatever's filtered. **Reports** breaks spending down by category and over time.
**Budget** is where monthly per-category limits are set and tracked. **Settings** holds account
credentials, interface language, currencies, categories, tags, and backup export/import.

## Architecture

The backend follows a hexagonal ("ports & adapters") architecture: business logic is organized as
one explicit **use case** per user action (e.g. `transaction/create`, `account/delete`,
`auth/updatecredentials`), each with its own `Input`/`Output`/`Execute`. Use cases depend only on
interfaces declared in `internal/port` — repositories, a password hasher, a token generator — never
on a concrete database driver or on `net/http`. Concrete implementations of those interfaces
("adapters") are plugged in once, at the composition root.

```
web/
  webassets.go              go:embed for the frontend (must live under web/, since go:embed can't
                             reach outside the directory of the file that declares it)
  index.html, css/          the UI shell and styling (Tailwind + Google Fonts, both via CDN)
  js/
    data/                   static reference data + i18n tables + the fetch() API wrapper
                             (accounts.js, categories.js, tags.js, transactions.js, i18n.js,
                             format.js, api.js)
    store/                  Vue reactive state: store.js, auth.js, router.js (hash-based),
                             notifications.js (e.g. "budget exceeded" toasts)
    components/             reusable pieces: modals for creating/editing each entity, chart
                             wrappers (Chart.js), icons, loading skeletons
    views/                  one file per top-level tab: login, dashboard, accounts, transactions,
                             reports, budget, settings
    app.js                  bootstraps the Vue app and wires router + store together

cmd/raccounting/
  main.go                   thin entry point; delegates everything to internal/app

internal/
  app/                      composition root: loads config, opens storage, wires every adapter and
                             use case together, starts the HTTP server, and the session-cleanup
                             goroutine
  config/                   env var loading & validation (app.go / db.go / log.go), fails fast on
                             anything malformed
  domain/
    entity/                 core types: Account, Transaction, Category, Currency, Tag, Budget,
                             User (+ PublicUser), Session, Backup — each with its own
                             MarshalJSON/UnmarshalJSON defining its wire shape
    error/                  typed errors — ValidationError, NotFoundError, ConflictError,
                             UnauthorizedError — mapped to HTTP status codes in one place
                             (internal/adapter/http/errors.go)
    usecase/                one directory per use case: account/, category/, currency/,
                             transaction/, transfer/, categorybudget/, tag/, settings/, data/, auth/
    service/
      passwordhasher/       bcrypt
      tokengenerator/       random session tokens
  port/                     the interfaces use cases depend on (repositories, PasswordHasher,
                             TokenGenerator), plus generated mocks for testing
  repository/                one package per entity (account, category, currency, transaction,
                              budget, tag, user, session); translates between domain entities and
                              storage models, turning sql.ErrNoRows / constraint violations into
                              typed domain errors. No SQL lives here — only calls into
                              internal/storage.
  storage/
    model/                  DB row shapes, kept separate from domain entities
    error/                  portable sentinels for constraint violations (unique, foreign key,
                             insufficient balance), since every driver reports them differently
  adapter/
    http/                   the driving adapter: net/http handlers, routing, cookies, JSON
                             encoding/decoding, and use-case-error → HTTP-status mapping
    sqlite/, mysql/, postgres/
                             one driven adapter per supported database: connection setup, goose
                             migrations (migrations/00001_init.sql, dialect-specific per adapter),
                             and Storage — all dialect-specific SQL lives here

build/docker/Dockerfile      minimal Alpine image that copies in a pre-built static binary
docker-compose.yml           a local MariaDB container for development
Makefile                     run / build / test / lint / docker-* targets
```

### Domain model

| Entity     | Fields |
|------------|--------|
| `User`     | id, username (unique), password (bcrypt hash), settings (`language`) |
| `Session`  | token (session cookie value), user id, expiry |
| `Currency` | code (ISO 4217, primary key), symbol, name, rate (relative to the default currency), default flag, archived |
| `Account`  | id, name, type (cash/card/account/savings/credit card/debt/virtual), currency, balance (minor units, never negative), archived |
| `Category` | id, name, color, type (income/expense), archived |
| `Tag`      | id, name, color |
| `Transaction` | id, type (income/expense/transfer), account, currency, amount (signed, minor units), category (income/expense only), tags, memo, operation date, transfer fields (see below) |
| `Budget`   | id, month (`YYYY-MM`), category, planned amount (minor units) |

A transaction's `Amount` is signed: negative for money leaving the account (an expense, or a
transfer's debit leg), positive for money arriving (income, or a transfer's credit leg). A transfer
is stored as two `Transaction` rows of type `transfer`, linked via `TransferTransactionID`, each
keeping its own account's currency; `TransferRate` records the exchange rate applied if the two
accounts' currencies differ. Every write that changes a balance — create, update, delete, on either
a transaction or a transfer — runs inside one DB transaction alongside the balance update itself,
and is rejected with a 409 if the result would be negative.

## Getting started

### Requirements

- Go 1.26+ to build from source (a prebuilt binary needs nothing but a database).
- One of:
  - MySQL or MariaDB, with a user that can create the database (or a database created ahead of
    time);
  - PostgreSQL, likewise;
  - or nothing at all — SQLite just needs a writable path for its database file.
- Docker, only if you want to run the bundled local MariaDB via `docker-compose.yml`.

### Configuration

Copy `.env.example` to `.env` and adjust it — both `docker compose` (for the local MariaDB
container) and the app itself (via [godotenv](https://github.com/joho/godotenv)) read it
automatically. Real environment variables always take priority over `.env`.

| Variable | Required | Default | Notes |
|---|---|---|---|
| `DB_DRIVER` | no | `mysql` | `mysql`, `postgres` or `sqlite` |
| `DB_HOST` | mysql/postgres only | — | |
| `DB_PORT` | mysql/postgres only | — | typically `3306` / `5432` |
| `DB_USER` | mysql/postgres only | — | |
| `DB_PASSWORD` | no | empty | read raw, not trimmed — a password may legitimately contain spaces |
| `DB_NAME` | mysql/postgres only | — | |
| `DB_SQLITE_PATH` | sqlite only | — | database file, created (with parent dirs) on first run |
| `DB_MAX_OPEN_CONNS` | no | `10` | ignored for sqlite (always 1 — no concurrent writers) |
| `DB_MAX_IDLE_CONNS` | no | `5` | ignored for sqlite |
| `DB_CONN_MAX_LIFETIME` | no | `5m` | Go duration syntax |
| `DB_CONN_TIMEOUT` | no | `5s` | initial TCP dial timeout, mysql only |
| `PORT` | no | `8080` | HTTP port the server listens on |
| `COOKIE_SECURE` | no | `false` | set `true` in production (HTTPS) so session cookies require TLS; a startup warning is logged while this is `false` |
| `LOG_CONSOLE_LEVEL` | no | `warn` | `debug` / `info` / `warn` / `error`, plain text to stdout |
| `LOG_FILE_PATH` | no | unset | JSON logs to a file, for a log aggregator; off unless set (nothing rotates it) |
| `LOG_FILE_LEVEL` | no | `info` | only used if `LOG_FILE_PATH` is set |
| `DEMO_USERNAME` / `DEMO_PASSWORD` | no | `user` / `pass` | the single demo account seeded on first run, only while the `users` table is still empty |

A value that's set but malformed (`PORT=http`, `COOKIE_SECURE=yes`, …) fails startup immediately
with a specific error, rather than silently falling back to a default.

### Run it

```bash
cp .env.example .env
docker compose up -d   # starts a local MariaDB container for development
go run ./cmd/raccounting
```

Migrations run automatically on startup (idempotent, via [goose](https://github.com/pressly/goose))
and a demo user is seeded the first time the `users` table is empty. Open
`http://localhost:8080` and log in with `DEMO_USERNAME` / `DEMO_PASSWORD` (`user` / `pass` by
default).

To run against SQLite instead — no Docker, no server needed:

```bash
DB_DRIVER=sqlite DB_SQLITE_PATH=./data/raccounting.db go run ./cmd/raccounting
```

If the configured MySQL/PostgreSQL user can't create databases, create it by hand first:

```sql
-- MySQL/MariaDB
CREATE DATABASE raccounting CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
-- PostgreSQL
CREATE DATABASE raccounting;
```

There's no registration screen — the app is single-user, and the only account is the one seeded
(or whatever credentials you change it to from Settings).

### Building & running the binary

```bash
go build -o raccounting ./cmd/raccounting
./raccounting
```

The binary already contains the entire frontend (`web/index.html`, `css/`, `js/`, `assets/`) via
`go:embed` — nothing else needs to ship alongside it besides a reachable database.

`make build` cross-compiles a static (`CGO_ENABLED=0`) Linux/amd64 binary into `build/app/`, ready
to be copied into the Alpine-based image at `build/docker/Dockerfile`.

### Docker image

```bash
make build                                   # produces build/app/raccounting
docker build -f build/docker/Dockerfile -t raccounting:latest .
docker run -p 8080:8080 --env-file .env raccounting:latest
```

`build/docker/Dockerfile` is a minimal `alpine:3.20` image (plus `ca-certificates`) that just copies
in the pre-built binary — the actual compilation happens on the host via `make build`, not inside
the image.

### Useful `make` targets

| Target | Does |
|---|---|
| `make run` | `go run ./cmd/raccounting` |
| `make build` | cross-compile a static binary into `build/app/raccounting` |
| `make test` | `go test ./...` |
| `make fmt` / `make vet` | `go fmt` / `go vet` |
| `make lint` | run `golangci-lint` (fetched into `./bin` by `make install-deps`) |
| `make docker-up` / `docker-down` / `docker-restart` / `docker-logs` / `docker-ps` | manage the local MariaDB container from `docker-compose.yml` |

Run `make help` for the full list.

## HTTP API

All responses are JSON; errors are `{"error": "<message>"}` with an appropriate HTTP status code
(400 validation / 401 unauthorized / 404 not found / 409 conflict / 500 internal). Every route but
`POST /api/auth/login` requires a valid session cookie and returns `401` without one.

| Method | Path | Description |
|---|---|---|
| GET | `/api/health` | liveness check |
| POST | `/api/auth/login` | `{username, password}` → sets the session cookie, returns the user |
| POST | `/api/auth/logout` | clears the session |
| GET | `/api/auth/me` | current user from the session cookie (used to restore a session after a page reload) |
| PATCH | `/api/auth/credentials` | change username/password, re-confirming the current password |
| GET | `/api/settings` | the signed-in user's saved UI settings (currently: `language`) |
| PATCH | `/api/settings` | update UI settings |
| GET | `/api/accounts` | list every account |
| POST | `/api/accounts` | create an account |
| PUT | `/api/accounts/{id}` | update (rename, recolor, archive) an account |
| DELETE | `/api/accounts/{id}` | delete an account |
| GET | `/api/categories` | list every category |
| POST | `/api/categories` | create a category |
| PUT | `/api/categories/{id}` | update a category |
| DELETE | `/api/categories/{id}` | delete a category |
| GET | `/api/currencies` | list every currency |
| POST | `/api/currencies` | create a currency |
| PUT | `/api/currencies/{code}` | update a currency (rate, default flag, archived) |
| DELETE | `/api/currencies/{code}` | delete a currency |
| GET | `/api/tags` | list every tag |
| POST | `/api/tags` | create a tag |
| PUT | `/api/tags/{id}` | update a tag |
| DELETE | `/api/tags/{id}` | delete a tag |
| GET | `/api/transactions` | one filtered/paginated page — query params for date range, account, category, tag, type, search, page, page size |
| GET | `/api/transactions/usage` | account/category ids referenced by at least one transaction — used to gate "can this be deleted?" in the UI |
| POST | `/api/transactions` | create an income/expense transaction |
| PUT | `/api/transactions/{id}` | update a transaction |
| DELETE | `/api/transactions/{id}` | delete a transaction |
| POST | `/api/transfers` | create a transfer between two accounts (both legs, atomically) |
| DELETE | `/api/transfers/{id}` | delete a transfer (both legs, atomically) |
| GET | `/api/category-budgets` | list budgeted amounts |
| PUT | `/api/category-budgets/{categoryId}/{monthKey}` | set (or, for a non-positive amount, clear) a category's budget for a `YYYY-MM` month |
| GET | `/api/data/export` | download the full backup snapshot as JSON |
| POST | `/api/data/import` | replace all data with a previously exported backup |

## Money handling

Every amount — account balances, transaction amounts, transfer amounts, budgeted amounts — is
stored and passed between backend layers as an integer in the currency's minor unit (e.g. cents),
never as a float, so nothing is ever lost to floating-point rounding. The frontend is responsible
for converting to/from the major-unit decimal a human types in. Currency conversion for the
dashboard's aggregate totals is likewise done client-side, using each currency's `rate` relative to
whichever one is marked default.

## Internationalization

The UI supports Russian, English, Spanish, German and French. Russian is the language the frontend
is actually written in — every UI string is a Russian literal at its call site — and the other four
languages are exact-match translation tables in `web/js/data/i18n.js`, keyed by the Russian source
string. A string missing from a table simply falls back to its Russian source text rather than
breaking. A handful of entries carry `{placeholders}` (e.g. `'Бюджет превышен: {name}'`) for
runtime-interpolated values. The chosen language is persisted both server-side
(`users.settings.language`, via `/api/settings`) and in `localStorage`, and is included in a backup
export/import so restoring one also restores the language it was made in.

## Testing

The Go codebase has extensive unit test coverage: every use case, every repository, every database
adapter (against `sqlmock` for MySQL/Postgres and a real SQLite file for that driver), and the HTTP
handlers. Mocks for the `internal/port` interfaces and each repository's storage dependency are
generated with `go.uber.org/mock`; `stretchr/testify` provides assertions and `brianvoe/gofakeit`
generates realistic test data. Run everything with:

```bash
go test ./...
# or
make test
```

## License

MIT — see [LICENSE](LICENSE).
