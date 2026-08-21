package postgres

import (
	"context"
	"database/sql/driver"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const currencySelectColumns = `code, symbol, name, rate, is_default, status`

func fakeCurrencyRow() model.Currency {
	return model.Currency{
		Code:    fakeCurrencyCode(),
		Symbol:  fakeSymbol(),
		Name:    fakeName(),
		Rate:    fakeRate(),
		Default: false,
		Status:  uint8(entity.CurrencyStatusActive),
	}
}

func currencyRowValues(m model.Currency) []driver.Value {
	return []driver.Value{m.Code, m.Symbol, m.Name, m.Rate, m.Default, m.Status}
}

// TestListCurrencies covers the full listing: every row comes back scanned into model.Currency.
func TestListCurrencies(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + currencySelectColumns + ` FROM currencies ORDER BY created_at`

	one := fakeCurrencyRow()
	two := fakeCurrencyRow()

	tests := []struct {
		name string
		rows []model.Currency
	}{
		{name: "an empty result yields no rows"},
		{name: "a single currency", rows: []model.Currency{one}},
		{name: "multiple currencies", rows: []model.Currency{one, two}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"})
			for _, c := range tt.rows {
				mockRows.AddRow(currencyRowValues(c)...)
			}

			mock.ExpectQuery(query).WillReturnRows(mockRows)

			got, err := s.ListCurrencies(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.rows, got)
		})
	}

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListCurrencies(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestExistsCurrency covers the existence check the account create path validates a referenced
// currency code against.
func TestExistsCurrency(t *testing.T) {
	t.Parallel()

	query := `SELECT COUNT(*) FROM currencies WHERE code = $1`
	code := fakeCurrencyCode()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "an existing code is found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(code).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown code is not found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(code).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
			},
			assertResult: func(t *testing.T, got bool) { require.False(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(code).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got bool) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.ExistsCurrency(context.Background(), code)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestCreateCurrency covers both write paths: a plain insert when Default is unset, and — when it's
// set — the atomic transaction that first clears every other currency's is_default flag. Either path
// wraps a taken code into storageError.UniqueViolationError via wrapUnique.
func TestCreateCurrency(t *testing.T) {
	t.Parallel()

	insertQuery := `INSERT INTO currencies (code, symbol, name, rate, is_default) VALUES ($1, $2, $3, $4, $5)`
	clearDefaultQuery := `UPDATE currencies SET is_default = FALSE`

	req := model.CurrencyCreateRequest{Code: fakeCurrencyCode(), Symbol: fakeSymbol(), Name: fakeName(), Rate: fakeRate()}

	t.Run("a non-default currency is a plain insert", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(insertQuery).WithArgs(req.Code, req.Symbol, req.Name, req.Rate, false).
			WillReturnResult(sqlmock.NewResult(0, 1))

		err := s.CreateCurrency(context.Background(), req)
		require.NoError(t, err)
	})

	t.Run("a taken code is a unique violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(insertQuery).WithArgs(req.Code, req.Symbol, req.Name, req.Rate, false).
			WillReturnError(pgErr(pgUniqueViolation))

		err := s.CreateCurrency(context.Background(), req)
		require.ErrorIs(t, err, storageError.UniqueViolationError)
	})

	defaultReq := req
	defaultReq.Default = true

	t.Run("a default currency clears every other one first, atomically", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(insertQuery).WithArgs(defaultReq.Code, defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		err := s.CreateCurrency(context.Background(), defaultReq)
		require.NoError(t, err)
	})

	t.Run("a failure clearing defaults rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnError(errStub)
		mock.ExpectRollback()

		err := s.CreateCurrency(context.Background(), defaultReq)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a taken code inside the default transaction rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(insertQuery).WithArgs(defaultReq.Code, defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true).
			WillReturnError(pgErr(pgUniqueViolation))
		mock.ExpectRollback()

		err := s.CreateCurrency(context.Background(), defaultReq)
		require.ErrorIs(t, err, storageError.UniqueViolationError)
	})
}

// TestUpdateCurrency mirrors TestCreateCurrency's two write paths for the update, plus the
// found/not-found translation and the follow-up read that produces the returned row.
func TestUpdateCurrency(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE currencies SET symbol = $1, name = $2, rate = $3, is_default = $4, status = $5,
			     updated_at = CURRENT_TIMESTAMP
			 WHERE code = $6`
	selectQuery := `SELECT ` + currencySelectColumns + ` FROM currencies WHERE code = $1`
	clearDefaultQuery := `UPDATE currencies SET is_default = FALSE`

	req := model.CurrencyUpdateRequest{
		Code: fakeCurrencyCode(), Symbol: fakeSymbol(), Name: fakeName(), Rate: fakeRate(), Archived: false,
	}
	updated := fakeCurrencyRow()
	updated.Code = req.Code

	t.Run("a non-default update returns the row", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).
			WithArgs(req.Symbol, req.Name, req.Rate, false, uint8(entity.CurrencyStatusActive), req.Code).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(selectQuery).WithArgs(req.Code).WillReturnRows(
			sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"}).
				AddRow(currencyRowValues(updated)...),
		)

		got, found, err := s.UpdateCurrency(context.Background(), req)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, updated, got)
	})

	t.Run("an unknown code is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).
			WithArgs(req.Symbol, req.Name, req.Rate, false, uint8(entity.CurrencyStatusActive), req.Code).
			WillReturnResult(sqlmock.NewResult(0, 0))

		_, found, err := s.UpdateCurrency(context.Background(), req)
		require.NoError(t, err)
		require.False(t, found)
	})

	defaultReq := req
	defaultReq.Default = true
	defaultUpdated := updated
	defaultUpdated.Default = true

	t.Run("a default update clears every other one first, atomically", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(updateQuery).
			WithArgs(defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true, uint8(entity.CurrencyStatusActive), defaultReq.Code).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectQuery(selectQuery).WithArgs(defaultReq.Code).WillReturnRows(
			sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"}).
				AddRow(currencyRowValues(defaultUpdated)...),
		)
		mock.ExpectCommit()

		got, found, err := s.UpdateCurrency(context.Background(), defaultReq)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, defaultUpdated, got)
	})

	t.Run("an unknown code inside the default transaction is not found, without committing", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(updateQuery).
			WithArgs(defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true, uint8(entity.CurrencyStatusActive), defaultReq.Code).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		_, found, err := s.UpdateCurrency(context.Background(), defaultReq)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a failure clearing defaults rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateCurrency(context.Background(), defaultReq)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})
}

// TestDeleteCurrency covers the delete and the FOREIGN KEY violation a still-referenced currency
// produces.
func TestDeleteCurrency(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM currencies WHERE code = $1`
	code := fakeCurrencyCode()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "deletes an unreferenced currency",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(code).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertResult: func(t *testing.T, found bool) { require.True(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown code is reported as not found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(code).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a currency still referenced by an account surfaces the driver's foreign key error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(code).WillReturnError(pgErr(pgForeignKeyViolation))
			},
			assertResult: func(t *testing.T, found bool) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.ForeignKeyViolationError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			found, err := s.DeleteCurrency(context.Background(), code)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
