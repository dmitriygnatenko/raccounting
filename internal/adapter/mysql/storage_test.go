package mysql

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
	mysqldriver "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"

	storageError "raccounting/internal/storage/error"
)

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

// mysqlErr builds a *mysqldriver.MySQLError carrying number — the field every switch in this
// package keys off — with a random message, since production code never inspects the message text.
func mysqlErr(number uint16) *mysqldriver.MySQLError {
	return &mysqldriver.MySQLError{
		Number:  number,
		Message: gofakeit.Sentence(),
	}
}

// fakeID returns a random id in a range that survives the uint64->int64 conversion database/sql
// applies to query arguments, so it's safe to bind directly.
func fakeID() uint64 { return uint64(gofakeit.Number(1, 1_000_000)) }

// fakeUsername, fakeHash, fakeName and fakeColor are the field-shaped random values the tests below
// bind into queries and mocked rows, so a test failure is never masked by two cases accidentally
// sharing a fixture value.
func fakeUsername() string { return gofakeit.Username() }
func fakeHash() string     { return gofakeit.LetterN(60) }
func fakeName() string     { return gofakeit.AppName() }
func fakeColor() string    { return gofakeit.HexColor() }
func fakeMemo() string     { return gofakeit.Sentence() }
func fakeCode() string     { return strings.ToUpper(gofakeit.Currency().Short) }
func fakeToken() string    { return gofakeit.UUID() }
func fakeSymbol() string   { return gofakeit.LetterN(1) }

// fakeAmount returns a random minor-unit amount, positive or negative depending on sign.
func fakeAmount() int64 { return int64(gofakeit.Number(1, 1_000_000)) }

// fakeRate returns a random exchange rate.
func fakeRate() float64 { return gofakeit.Price(0.01, 1000) }

// fakeTime returns a random timestamp truncated to the second — MySQL's TIMESTAMP columns (see
// migrate.go) don't carry fractional seconds, so keeping sub-second precision out of the fixture
// avoids tests asserting a precision the schema doesn't have.
func fakeTime() time.Time { return gofakeit.Date().UTC().Truncate(time.Second) }

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

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

// TestInsertReturningID checks that the generated id comes back from LastInsertId, and that a
// failing statement reports the error instead of a zero id.
func TestInsertReturningID(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO tags (name, color) VALUES (?, ?)`
	name, color := fakeName(), fakeColor()
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "the id comes back from LastInsertId",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(name, color).WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a failing statement returns its error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(name, color).WillReturnError(errStub)
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

// TestWrapUnique covers the one error shape the repositories are allowed to recognize: a duplicate
// entry (MySQL error 1062), normalized to storageError.UniqueViolationError with the driver error
// still wrapped inside. Every other error — including another constraint failure — has to pass
// through untouched, otherwise a repository would report "already taken" for an unrelated failure.
func TestWrapUnique(t *testing.T) {
	t.Parallel()

	fkViolation := mysqlErr(errRowIsReferenced)
	duplicate := mysqlErr(errDuplicateEntry)

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
			name: "a duplicate entry is wrapped",
			err:  duplicate,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.UniqueViolationError)

				var driverErr *mysqldriver.MySQLError
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

// TestWrapForeignKey covers the one error shape a DELETE/UPDATE blocked by a referencing row can
// take: MySQL error 1451, normalized to storageError.ForeignKeyViolationError. Every other error —
// including a duplicate entry — has to pass through untouched.
func TestWrapForeignKey(t *testing.T) {
	t.Parallel()

	duplicate := mysqlErr(errDuplicateEntry)
	referenced := mysqlErr(errRowIsReferenced)

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
			name: "a referenced row is wrapped",
			err:  referenced,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.ForeignKeyViolationError)

				var driverErr *mysqldriver.MySQLError
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

// TestWrapInsufficientBalance covers both MySQL error numbers a write taking the unsigned
// accounts.balance column negative can surface as — see the doc comment on errWarnDataOutOfRange in
// storage.go for why there are two. Every other error has to pass through untouched.
func TestWrapInsufficientBalance(t *testing.T) {
	t.Parallel()

	warnOutOfRange := mysqlErr(errWarnDataOutOfRange)
	outOfRange := mysqlErr(errDataOutOfRange)
	duplicate := mysqlErr(errDuplicateEntry)

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
			name: "a negative literal assignment (ER_WARN_DATA_OUT_OF_RANGE) is wrapped",
			err:  warnOutOfRange,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.InsufficientBalanceError)
			},
		},
		{
			name: "an arithmetic underflow (ER_DATA_OUT_OF_RANGE) is wrapped",
			err:  outOfRange,
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.InsufficientBalanceError)
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
// name is validated — it's spliced directly into a CREATE DATABASE `%s` statement (see the
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
		{
			name:   "a plain identifier passes the validator",
			dbName: gofakeit.Word() + "_" + gofakeit.LetterN(6),
		},
		{
			name:             "a name with a space is rejected",
			dbName:           gofakeit.Word() + " " + gofakeit.Word(),
			wantRejectedName: true,
		},
		{
			name:             "a name with a semicolon is rejected",
			dbName:           gofakeit.Word() + "; DROP DATABASE mysql",
			wantRejectedName: true,
		},
		{
			name:             "a name with a backtick is rejected",
			dbName:           "`" + gofakeit.Word(),
			wantRejectedName: true,
		},
		{
			name:             "an empty name is rejected",
			dbName:           "",
			wantRejectedName: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cfg := Config{
				Host:        "127.0.0.1",
				Port:        closedPort(t),
				Name:        tt.dbName,
				ConnTimeout: 200 * time.Millisecond,
			}

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
		Host:        "127.0.0.1",
		Port:        closedPort(t),
		User:        fakeUsername(),
		Password:    fakeHash(),
		Name:        gofakeit.Word(),
		ConnTimeout: 200 * time.Millisecond,
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
