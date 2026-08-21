package postgres

import (
	"context"
	"database/sql"
	"errors"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"

	storageError "raccounting/internal/storage/error"
)

// pgOtherCheckConstraint is the auto-generated name of a CHECK constraint this schema has besides
// accounts' balance one (categories.type, transactions.type/status, accounts.type all share
// SQLSTATE 23514) — used to prove wrapInsufficientBalance discriminates on constraint name, not just
// the SQLSTATE.
const pgOtherCheckConstraint = "categories_type_check"

// newMock returns a Storage backed by sqlmock instead of a real server: every query the adapter
// issues has to be told what to expect ahead of time, and the test fails on any call that wasn't
// expected or any expectation that went unmet.
func newMock(t *testing.T) (*Storage, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	require.NoError(t, err)

	t.Cleanup(func() { _ = db.Close() })
	t.Cleanup(func() {
		require.NoError(t, mock.ExpectationsWereMet(), "unmet sqlmock expectations")
	})

	return &Storage{DB: db}, mock
}

// pgErr builds a *pgconn.PgError carrying code — the field every switch in this package keys off —
// with a random message, since production code never inspects the message text.
func pgErr(code string) *pgconn.PgError {
	return &pgconn.PgError{Code: code, Message: gofakeit.Sentence()}
}

// fakeID returns a random id in a range that survives the uint64->int64 conversion database/sql
// applies to query arguments, so it's safe to bind directly.
func fakeID() uint64 { return uint64(gofakeit.Number(1, 1_000_000)) }

// fakeUsername, fakeHash, fakeName, fakeMemo, fakeColor, fakeToken, fakeCurrencyCode, fakeSymbol and
// fakeMonthKey are the field-shaped random values the tests below bind into queries and mocked rows,
// so a test failure is never masked by two cases accidentally sharing a fixture value.
func fakeUsername() string     { return gofakeit.Username() }
func fakeHash() string         { return gofakeit.LetterN(60) }
func fakeName() string         { return gofakeit.AppName() }
func fakeMemo() string         { return gofakeit.Sentence() }
func fakeColor() string        { return gofakeit.HexColor() }
func fakeToken() string        { return gofakeit.UUID() }
func fakeCurrencyCode() string { return strings.ToUpper(gofakeit.Currency().Short) }
func fakeSymbol() string       { return gofakeit.LetterN(3) }
func fakeMonthKey() string     { return gofakeit.Date().Format("2006-01") }
func fakeRate() float64        { return gofakeit.Price(0.01, 1000) }
func fakeAmount() int64        { return int64(gofakeit.Number(1, 1_000_000)) }

// fakeTime returns a random timestamp truncated to the microsecond — Postgres's TIMESTAMP columns
// (see migrate.go) don't carry sub-microsecond precision, so keeping nanosecond precision out of the
// fixture avoids tests asserting a precision the schema doesn't have.
func fakeTime() time.Time { return gofakeit.Date().UTC().Truncate(time.Microsecond) }

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// balanceCheckErr returns a *pgconn.PgError shaped like the one Postgres raises when a write takes
// accounts.balance below zero — the specific check_violation/constraint-name pair
// wrapInsufficientBalance recognizes, used by every entity test that exercises that path.
func balanceCheckErr() *pgconn.PgError {
	err := pgErr(pgCheckViolation)
	err.ConstraintName = pgBalanceCheckConstraint

	return err
}

// TestAffected pins the translation from a driver Result to the "found?" answer the storage layer
// gives updates and deletes.
func TestAffected(t *testing.T) {
	t.Parallel()

	rowsTouched := int64(gofakeit.Number(1, 1000))

	type args struct {
		res sql.Result
	}

	tests := []struct {
		name         string
		args         args
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name:         "no rows touched means not found",
			args:         args{res: sqlmock.NewResult(0, 0)},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "rows touched means found",
			args:         args{res: sqlmock.NewResult(0, rowsTouched)},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "the driver error is propagated",
			args:         args{res: sqlmock.NewErrorResult(errStub)},
			assertResult: func(t *testing.T, got bool) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := affected(tt.args.res)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestInsertReturningID checks that the generated id comes back from the RETURNING id clause —
// Postgres drivers don't support LastInsertId, so insertReturningID scans it out of a query result
// instead of an exec result — and that a failing statement reports the error instead of a zero id.
func TestInsertReturningID(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO tags (name, color) VALUES ($1, $2) RETURNING id`
	name, color := fakeName(), fakeColor()
	wantID := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "the id comes back from the RETURNING clause",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).
					WithArgs(name, color).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, wantID, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a failing statement returns its error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).
					WithArgs(name, color).
					WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.insertReturningID(context.Background(), query, name, color)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestWrapUnique covers the one error shape the repositories are allowed to recognize: a
// unique_violation (SQLSTATE 23505), normalized to storageError.UniqueViolationError with the driver
// error still wrapped inside. Every other error — including another constraint failure — has to pass
// through untouched, otherwise a repository would report "already taken" for an unrelated failure.
func TestWrapUnique(t *testing.T) {
	t.Parallel()

	fkViolation := pgErr(pgForeignKeyViolation)
	duplicate := pgErr(pgUniqueViolation)

	tests := []struct {
		name         string
		err          error
		assertResult func(t *testing.T, in error, got error)
	}{
		{
			name: "nil passes through",
			err:  nil,
			assertResult: func(t *testing.T, in error, got error) {
				require.NoError(t, got)
			},
		},
		{
			name: "an unrelated error passes through",
			err:  errStub,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.UniqueViolationError)
			},
		},
		{
			name: "another constraint failure passes through",
			err:  fkViolation,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.UniqueViolationError)
			},
		},
		{
			name: "a unique violation is wrapped",
			err:  duplicate,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.UniqueViolationError)

				var driverErr *pgconn.PgError
				require.ErrorAs(t, got, &driverErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := wrapUnique(tt.err)
			tt.assertResult(t, tt.err, got)
		})
	}
}

// TestWrapForeignKey covers the one error shape the repositories are allowed to recognize: a
// foreign_key_violation (SQLSTATE 23503), normalized to storageError.ForeignKeyViolationError with
// the driver error still wrapped inside. Every other error — including a unique violation — has to
// pass through untouched.
func TestWrapForeignKey(t *testing.T) {
	t.Parallel()

	duplicate := pgErr(pgUniqueViolation)
	fkViolation := pgErr(pgForeignKeyViolation)

	tests := []struct {
		name         string
		err          error
		assertResult func(t *testing.T, in error, got error)
	}{
		{
			name: "nil passes through",
			err:  nil,
			assertResult: func(t *testing.T, in error, got error) {
				require.NoError(t, got)
			},
		},
		{
			name: "an unrelated error passes through",
			err:  errStub,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.ForeignKeyViolationError)
			},
		},
		{
			name: "another constraint failure passes through",
			err:  duplicate,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.ForeignKeyViolationError)
			},
		},
		{
			name: "a foreign key violation is wrapped",
			err:  fkViolation,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.ForeignKeyViolationError)

				var driverErr *pgconn.PgError
				require.ErrorAs(t, got, &driverErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := wrapForeignKey(tt.err)
			tt.assertResult(t, tt.err, got)
		})
	}
}

// TestWrapInsufficientBalance covers the one error shape the repositories are allowed to recognize:
// a check_violation (SQLSTATE 23514) specifically on the accounts.balance CHECK constraint,
// normalized to storageError.InsufficientBalanceError. This schema has other CHECK constraints too
// (categories.type, transactions.type/status, accounts.type), all sharing SQLSTATE 23514, so a check
// violation on any of those must pass through untouched — the constraint name is what disambiguates.
func TestWrapInsufficientBalance(t *testing.T) {
	t.Parallel()

	duplicate := pgErr(pgUniqueViolation)

	otherCheck := pgErr(pgCheckViolation)
	otherCheck.ConstraintName = pgOtherCheckConstraint

	balanceCheck := balanceCheckErr()

	tests := []struct {
		name         string
		err          error
		assertResult func(t *testing.T, in error, got error)
	}{
		{
			name: "nil passes through",
			err:  nil,
			assertResult: func(t *testing.T, in error, got error) {
				require.NoError(t, got)
			},
		},
		{
			name: "an unrelated error passes through",
			err:  errStub,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.InsufficientBalanceError)
			},
		},
		{
			name: "another constraint failure passes through",
			err:  duplicate,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.InsufficientBalanceError)
			},
		},
		{
			name: "a check violation on a different constraint passes through",
			err:  otherCheck,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.InsufficientBalanceError)
			},
		},
		{
			name: "the balance check violation is wrapped",
			err:  balanceCheck,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.InsufficientBalanceError)

				var driverErr *pgconn.PgError
				require.ErrorAs(t, got, &driverErr)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := wrapInsufficientBalance(tt.err)
			tt.assertResult(t, tt.err, got)
		})
	}
}

// TestEnsureDatabase covers the one thing it can without a real server to create a database on: the
// name is validated — it's spliced directly into a CREATE DATABASE "%s" statement (see the
// #nosec-worthy comment that would need in storage.go if identifierRE didn't guard it first) — before
// any connection is attempted. A name that isn't a bare identifier has to be rejected at that check,
// distinguished here from a name that passes it and fails later for the mundane reason that nothing
// is listening on the port.
func TestEnsureDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		dbName           string
		wantRejectedName bool
	}{
		{name: "a plain identifier passes the validator", dbName: gofakeit.Word() + "_" + gofakeit.LetterN(6)},
		{name: "a name with a space is rejected", dbName: gofakeit.Word() + " " + gofakeit.Word(), wantRejectedName: true},
		{
			name:             "a name with a semicolon is rejected",
			dbName:           gofakeit.Word() + "; DROP DATABASE postgres",
			wantRejectedName: true,
		},
		{name: "a name with a quote is rejected", dbName: `"` + gofakeit.Word(), wantRejectedName: true},
		{name: "an empty name is rejected", dbName: "", wantRejectedName: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{Host: "127.0.0.1", Port: closedPort(t), Name: tt.dbName}

			err := EnsureDatabase(cfg)
			require.Error(t, err, "want an error either way")
			require.Equal(t, tt.wantRejectedName, strings.Contains(err.Error(), "invalid database name"))
		})
	}
}

// TestOpen checks that an unreachable server is reported at Open (which pings) rather than at the
// first query — a lazily-connecting driver would otherwise stay silent about it. A live server isn't
// needed for this: a closed local port refuses the connection deterministically everywhere.
func TestOpen(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Host: "127.0.0.1", Port: closedPort(t), User: fakeUsername(), Password: fakeHash(), Name: gofakeit.Word(),
	}

	s, err := Open(cfg)
	if err == nil {
		_ = s.Close()
	}

	require.Error(t, err, "want an error for an unreachable server")
}

// closedPort returns a TCP port on localhost that nothing is listening on, by opening then
// immediately closing a listener — deterministic and available in any environment, unlike a
// reserved-looking magic number that might collide with something already running.
func closedPort(t *testing.T) string {
	t.Helper()

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "finding a free port")

	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	return strconv.Itoa(port)
}
