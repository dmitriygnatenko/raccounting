package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	"modernc.org/sqlite"

	storageError "raccounting/internal/storage/error"
)

// newMock returns a Storage backed by sqlmock instead of a real database file: every query the
// adapter issues has to be told what to expect ahead of time, and the test fails on any call that
// wasn't expected or any expectation that went unmet.
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

// sqliteUniqueErr, sqliteForeignKeyErr and sqliteInsufficientBalanceErr return real *sqlite.Error
// values for the three constraint violations these tests need to simulate. modernc.org/sqlite
// doesn't export a constructor for its Error type (its fields are private) — running a real
// statement engineered to fail is the only way to get one, so primeSQLiteErrs mints all three once,
// against throwaway in-memory databases, and every test reuses the result.
var (
	sqliteErrOnce                 sync.Once
	sqliteUniqueErrVal            *sqlite.Error
	sqliteForeignKeyErrVal        *sqlite.Error
	sqliteInsufficientBalancesVal *sqlite.Error
)

func primeSQLiteErrs() {
	db, err := sql.Open("sqlite", "file::memory:?_pragma=foreign_keys(1)")
	if err != nil {
		panic(err)
	}
	defer db.Close()

	if _, err := db.Exec(`CREATE TABLE parent (id INTEGER PRIMARY KEY)`); err != nil {
		panic(err)
	}

	if _, err := db.Exec(`CREATE TABLE child (
		id INTEGER PRIMARY KEY, parent_id INTEGER REFERENCES parent (id)
	)`); err != nil {
		panic(err)
	}

	if _, err := db.Exec(`CREATE TABLE uniq (val INTEGER UNIQUE)`); err != nil {
		panic(err)
	}

	if _, err := db.Exec(`CREATE TABLE balances (id INTEGER PRIMARY KEY, balance INTEGER NOT NULL CHECK (balance >= 0))`); err != nil {
		panic(err)
	}

	if _, err := db.Exec(`INSERT INTO child (id, parent_id) VALUES (1, 4242)`); !errors.As(err, &sqliteForeignKeyErrVal) {
		panic("expected a *sqlite.Error for the foreign key violation, got " + errString(err))
	}

	if _, err := db.Exec(`INSERT INTO uniq (val) VALUES (1)`); err != nil {
		panic(err)
	}

	if _, err := db.Exec(`INSERT INTO uniq (val) VALUES (1)`); !errors.As(err, &sqliteUniqueErrVal) {
		panic("expected a *sqlite.Error for the unique violation, got " + errString(err))
	}

	if _, err := db.Exec(`INSERT INTO balances (id, balance) VALUES (1, 10)`); err != nil {
		panic(err)
	}

	if _, err := db.Exec(`UPDATE balances SET balance = balance - 20 WHERE id = 1`); !errors.As(err, &sqliteInsufficientBalancesVal) {
		panic("expected a *sqlite.Error for the balance CHECK violation, got " + errString(err))
	}
}

func errString(err error) string {
	if err == nil {
		return "<nil>"
	}

	return err.Error()
}

// sqliteUniqueErr is the *sqlite.Error a UNIQUE constraint violation surfaces as.
func sqliteUniqueErr() *sqlite.Error {
	sqliteErrOnce.Do(primeSQLiteErrs)

	return sqliteUniqueErrVal
}

// sqliteForeignKeyErr is the *sqlite.Error a foreign key violation surfaces as — both an insert/update
// referencing a row that doesn't exist, and a delete blocked by ON DELETE RESTRICT.
func sqliteForeignKeyErr() *sqlite.Error {
	sqliteErrOnce.Do(primeSQLiteErrs)

	return sqliteForeignKeyErrVal
}

// sqliteInsufficientBalanceErr is the *sqlite.Error the accounts.balance CHECK (balance >= 0)
// constraint surfaces as when a write would take it negative.
func sqliteInsufficientBalanceErr() *sqlite.Error {
	sqliteErrOnce.Do(primeSQLiteErrs)

	return sqliteInsufficientBalancesVal
}

// fakeID returns a random id in a range that survives the uint64->int64 conversion database/sql
// applies to query arguments, so it's safe to bind directly.
func fakeID() uint64 { return uint64(gofakeit.Number(1, 1_000_000)) }

// fakeName, fakeColor, fakeUsername, fakeHash, fakeToken, fakeCurrencyCode, fakeMemo, fakeMonthKey,
// fakeAmount and fakeRate are the field-shaped random values the tests below bind into queries and
// mocked rows, so a test failure is never masked by two cases accidentally sharing a fixture value.
func fakeName() string         { return gofakeit.AppName() }
func fakeColor() string        { return gofakeit.HexColor() }
func fakeUsername() string     { return gofakeit.Username() }
func fakeHash() string         { return gofakeit.LetterN(60) }
func fakeToken() string        { return gofakeit.UUID() }
func fakeCurrencyCode() string { return gofakeit.CurrencyShort() }
func fakeMemo() string         { return gofakeit.Sentence(6) }
func fakeMonthKey() string     { return gofakeit.Date().Format("2006-01") }
func fakeAmount() int64        { return int64(gofakeit.Number(1, 1_000_000)) }
func fakeRate() float64        { return gofakeit.Price(0.01, 100) }

// fakeTime returns a random timestamp. Unlike MySQL's TIMESTAMP columns, SQLite's TEXT-backed
// timestamps carry full time.Time precision, so — unlike the mysql adapter's equivalent — there's no
// need to truncate to the second here.
func fakeTime() time.Time { return gofakeit.Date().UTC() }

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

// TestInsertReturningID checks that the generated id comes back from LastInsertId, and that a failing
// statement reports the error instead of a zero id.
func TestInsertReturningID(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO categories (name, type, color) VALUES (?, ?, ?)`
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
				mock.ExpectExec(query).
					WithArgs(name, uint8(1), color).
					WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a failing statement returns its error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).
					WithArgs(name, uint8(1), color).
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

			got, err := s.insertReturningID(context.Background(), query, name, uint8(1), color)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestWrapUnique covers the one error shape the repositories are allowed to recognize as "taken": a
// UNIQUE violation, normalized to storageError.UniqueViolationError with the driver error still
// wrapped inside. Every other error — including another SQLite constraint failure — has to pass
// through untouched, otherwise a repository would report "already taken" for an unrelated failure.
func TestWrapUnique(t *testing.T) {
	t.Parallel()

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
			err:  sqliteForeignKeyErr(),
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.UniqueViolationError)
			},
		},
		{
			name: "a unique violation is wrapped",
			err:  sqliteUniqueErr(),
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.UniqueViolationError)

				var driverErr *sqlite.Error
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

// TestWrapForeignKey covers the one error shape the repositories are allowed to recognize as
// "still in use": a FOREIGN KEY violation, normalized to storageError.ForeignKeyViolationError with
// the driver error still wrapped inside. Every other error — including another constraint failure —
// has to pass through untouched.
func TestWrapForeignKey(t *testing.T) {
	t.Parallel()

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
			err:  sqliteUniqueErr(),
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.ForeignKeyViolationError)
			},
		},
		{
			name: "a foreign key violation is wrapped",
			err:  sqliteForeignKeyErr(),
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.ForeignKeyViolationError)

				var driverErr *sqlite.Error
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

// TestWrapInsufficientBalance covers the one error shape that means "this write would overdraw the
// account": the accounts.balance CHECK (balance >= 0) constraint failing, normalized to
// storageError.InsufficientBalanceError. A CHECK failure on an unrelated column (e.g. accounts.type)
// has to pass through untouched — wrapInsufficientBalance matches on "CHECK constraint failed:
// balance" specifically, not any CHECK failure.
func TestWrapInsufficientBalance(t *testing.T) {
	t.Parallel()

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
			err:  sqliteForeignKeyErr(),
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.NotErrorIs(t, got, storageError.InsufficientBalanceError)
			},
		},
		{
			name: "a balance CHECK violation is wrapped",
			err:  sqliteInsufficientBalanceErr(),
			assertResult: func(t *testing.T, in error, got error) {
				require.ErrorIs(t, got, in)
				require.ErrorIs(t, got, storageError.InsufficientBalanceError)

				var driverErr *sqlite.Error
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

// TestEnsureDatabase covers the one thing it promises: the parent directory exists afterwards. The
// database file itself is the driver's job, on first connection.
func TestEnsureDatabase(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		setup     func(t *testing.T, tmp string)
		path      func(tmp string) string
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "a missing parent directory is created",
			path: func(tmp string) string { return filepath.Join(tmp, "data", "nested", "app.db") },
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "an existing parent directory is left alone",
			path: func(tmp string) string { return filepath.Join(tmp, "app.db") },
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "a bare filename has no directory to create",
			path: func(tmp string) string { return "app.db" },
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "a file where the parent directory should be is an error",
			setup: func(t *testing.T, tmp string) {
				t.Helper()

				require.NoError(t, os.WriteFile(filepath.Join(tmp, "data"), nil, 0o600))
			},
			path: func(tmp string) string { return filepath.Join(tmp, "data", "app.db") },
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			tmp := t.TempDir()
			if tt.setup != nil {
				tt.setup(t, tmp)
			}

			path := tt.path(tmp)

			err := EnsureDatabase(Config{Path: path})
			tt.assertErr(t, err)

			if err != nil {
				return
			}

			info, statErr := os.Stat(filepath.Dir(path))
			require.NoError(t, statErr)
			require.True(t, info.IsDir())
		})
	}
}

// TestOpen checks that a usable pool comes back for a workable path, that the single-connection cap
// is applied, and that an unusable path fails at Open rather than at the first query — Open pings,
// which is where a lazily-connecting driver would otherwise stay silent.
func TestOpen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		path      func(tmp string) string
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "creates and opens the database file",
			path: func(tmp string) string { return filepath.Join(tmp, "app.db") },
			assertErr: func(t *testing.T, err error) {
				require.NoError(t, err)
			},
		},
		{
			name: "a directory in place of the file is an error",
			path: func(tmp string) string { return tmp },
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
		{
			name: "a missing parent directory is an error",
			path: func(tmp string) string { return filepath.Join(tmp, "missing", "app.db") },
			assertErr: func(t *testing.T, err error) {
				require.Error(t, err)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := tt.path(t.TempDir())

			s, err := Open(Config{
				Path:            path,
				ConnMaxLifetime: time.Minute,
			})
			tt.assertErr(t, err)

			if err != nil {
				return
			}

			defer func() { _ = s.Close() }()

			require.Equal(t, 1, s.Stats().MaxOpenConnections)

			_, statErr := os.Stat(path)
			require.NoError(t, statErr, "Open() did not create the file")
		})
	}
}
