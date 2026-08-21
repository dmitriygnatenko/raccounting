package sqlite

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const currencySelectQuery = `SELECT code, symbol, name, rate, is_default, status FROM currencies`

var currencyColumnNames = []string{"code", "symbol", "name", "rate", "is_default", "status"}

const (
	clearDefaultQuery   = `UPDATE currencies SET is_default = 0`
	createQuery         = `INSERT INTO currencies (code, symbol, name, rate, is_default) VALUES (?, ?, ?, ?, ?)`
	updateQueryCurrency = `UPDATE currencies SET symbol = ?, name = ?, rate = ?, is_default = ?, status = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE code = ?`
	deleteCurrencyQuery = `DELETE FROM currencies WHERE code = ?`
)

// fakeCurrency returns a random model.Currency, as ListCurrencies/updateCurrency scan it.
func fakeCurrency() model.Currency {
	return model.Currency{
		Code:    fakeCurrencyCode(),
		Symbol:  fakeName(),
		Name:    fakeName(),
		Rate:    fakeRate(),
		Default: false,
		Status:  uint8(entity.CurrencyStatusActive),
	}
}

func addCurrencyRow(rows *sqlmock.Rows, m model.Currency) *sqlmock.Rows {
	return rows.AddRow(m.Code, m.Symbol, m.Name, m.Rate, m.Default, m.Status)
}

// TestListCurrencies covers the full listing: every row comes back scanned into model.Currency.
func TestListCurrencies(t *testing.T) {
	t.Parallel()

	query := currencySelectQuery + ` ORDER BY created_at`

	c1, c2 := fakeCurrency(), fakeCurrency()

	tests := []struct {
		name string
		rows []model.Currency
	}{
		{name: "an empty result yields no rows"},
		{name: "a single currency", rows: []model.Currency{c1}},
		{name: "several currencies", rows: []model.Currency{c1, c2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows(currencyColumnNames)
			for _, c := range tt.rows {
				addCurrencyRow(mockRows, c)
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

// TestExistsCurrency covers the referential check used before pointing an account at a currency
// code.
func TestExistsCurrency(t *testing.T) {
	t.Parallel()

	query := `SELECT COUNT(*) FROM currencies WHERE code = ?`
	code := fakeCurrencyCode()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "an existing code reports true",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(code).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			},
			assertResult: func(t *testing.T, got bool) { require.True(t, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown code reports false",
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

// TestCreateCurrency covers both branches: a non-default currency is a single insert, while a
// default one runs inside a transaction that clears every other row's is_default flag first.
func TestCreateCurrency(t *testing.T) {
	t.Parallel()

	req := model.CurrencyCreateRequest{
		Code:   fakeCurrencyCode(),
		Symbol: fakeName(),
		Name:   fakeName(),
		Rate:   fakeRate(),
	}

	t.Run("a non-default currency is a single insert", func(t *testing.T) {
		t.Parallel()

		tests := []struct {
			name      string
			mockErr   error
			assertErr func(t *testing.T, err error)
		}{
			{
				name:      "inserts the currency",
				assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
			},
			{
				name:    "a taken code is a unique violation",
				mockErr: sqliteUniqueErr(),
				assertErr: func(t *testing.T, err error) {
					require.ErrorIs(t, err, storageError.UniqueViolationError)
				},
			},
			{
				name:      "a driver error is propagated",
				mockErr:   errStub,
				assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()

				s, mock := newMock(t)

				exp := mock.ExpectExec(createQuery).WithArgs(req.Code, req.Symbol, req.Name, req.Rate, false)
				if tt.mockErr != nil {
					exp.WillReturnError(tt.mockErr)
				} else {
					exp.WillReturnResult(sqlmock.NewResult(0, 1))
				}

				err := s.CreateCurrency(context.Background(), req)
				tt.assertErr(t, err)
			})
		}
	})

	t.Run("a default currency clears every other row first, atomically", func(t *testing.T) {
		t.Parallel()

		defaultReq := req
		defaultReq.Default = true

		t.Run("clears the flag then inserts, and commits", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectBegin()
			mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
			mock.ExpectExec(createQuery).WithArgs(defaultReq.Code, defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectCommit()

			err := s.CreateCurrency(context.Background(), defaultReq)
			require.NoError(t, err)
		})

		t.Run("a failure clearing the flag rolls back", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectBegin()
			mock.ExpectExec(clearDefaultQuery).WillReturnError(errStub)
			mock.ExpectRollback()

			err := s.CreateCurrency(context.Background(), defaultReq)
			require.ErrorIs(t, err, errStub)
		})

		t.Run("a taken code rolls back and is a unique violation", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectBegin()
			mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
			mock.ExpectExec(createQuery).WithArgs(defaultReq.Code, defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true).
				WillReturnError(sqliteUniqueErr())
			mock.ExpectRollback()

			err := s.CreateCurrency(context.Background(), defaultReq)
			require.ErrorIs(t, err, storageError.UniqueViolationError)
		})
	})
}

// TestUpdateCurrency covers both branches: a non-default update runs directly against the pool,
// while a default one runs inside a transaction that clears every other row's is_default flag
// first. Either way, the full updated row is re-read on success, and an unknown code is reported as
// not found without a re-read.
func TestUpdateCurrency(t *testing.T) {
	t.Parallel()

	req := model.CurrencyUpdateRequest{
		Code:   fakeCurrencyCode(),
		Symbol: fakeName(),
		Name:   fakeName(),
		Rate:   fakeRate(),
	}
	selectQuery := currencySelectQuery + ` WHERE code = ?`

	t.Run("a non-default update runs directly against the pool", func(t *testing.T) {
		t.Parallel()

		nonDefaultReq := req
		nonDefaultReq.Default = false

		want := fakeCurrency()
		want.Code = req.Code

		t.Run("updates and re-reads the row", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectExec(updateQueryCurrency).
				WithArgs(nonDefaultReq.Symbol, nonDefaultReq.Name, nonDefaultReq.Rate, false,
					uint8(entity.CurrencyStatusActive), nonDefaultReq.Code).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(selectQuery).WithArgs(nonDefaultReq.Code).WillReturnRows(
				addCurrencyRow(sqlmock.NewRows(currencyColumnNames), want),
			)

			got, found, err := s.UpdateCurrency(context.Background(), nonDefaultReq)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, want, got)
		})

		t.Run("an unknown code is reported as not found, no re-read", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectExec(updateQueryCurrency).
				WithArgs(nonDefaultReq.Symbol, nonDefaultReq.Name, nonDefaultReq.Rate, false,
					uint8(entity.CurrencyStatusActive), nonDefaultReq.Code).
				WillReturnResult(sqlmock.NewResult(0, 0))

			_, found, err := s.UpdateCurrency(context.Background(), nonDefaultReq)
			require.NoError(t, err)
			require.False(t, found)
		})
	})

	t.Run("a default update clears every other row first, atomically", func(t *testing.T) {
		t.Parallel()

		defaultReq := req
		defaultReq.Default = true

		want := fakeCurrency()
		want.Code = req.Code
		want.Default = true

		t.Run("clears the flag, updates, re-reads, and commits", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectBegin()
			mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
			mock.ExpectExec(updateQueryCurrency).
				WithArgs(defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true,
					uint8(entity.CurrencyStatusActive), defaultReq.Code).
				WillReturnResult(sqlmock.NewResult(0, 1))
			mock.ExpectQuery(selectQuery).WithArgs(defaultReq.Code).WillReturnRows(
				addCurrencyRow(sqlmock.NewRows(currencyColumnNames), want),
			)
			mock.ExpectCommit()

			got, found, err := s.UpdateCurrency(context.Background(), defaultReq)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, want, got)
		})

		t.Run("an unknown code rolls back without committing", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectBegin()
			mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
			mock.ExpectExec(updateQueryCurrency).
				WithArgs(defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true,
					uint8(entity.CurrencyStatusActive), defaultReq.Code).
				WillReturnResult(sqlmock.NewResult(0, 0))
			mock.ExpectRollback()

			_, found, err := s.UpdateCurrency(context.Background(), defaultReq)
			require.NoError(t, err)
			require.False(t, found)
		})

		t.Run("a failure clearing the flag rolls back", func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mock.ExpectBegin()
			mock.ExpectExec(clearDefaultQuery).WillReturnError(errStub)
			mock.ExpectRollback()

			_, found, err := s.UpdateCurrency(context.Background(), defaultReq)
			require.ErrorIs(t, err, errStub)
			require.False(t, found)
		})
	})
}

// TestDeleteCurrency covers the delete and the constraint failure a currency still referenced by an
// account produces.
func TestDeleteCurrency(t *testing.T) {
	t.Parallel()

	code := fakeCurrencyCode()

	tests := []struct {
		name         string
		res          sql.Result
		mockErr      error
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name:         "deletes an unreferenced currency",
			res:          sqlmock.NewResult(0, 1),
			assertResult: func(t *testing.T, found bool) { require.True(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "an unknown code is reported as not found",
			res:          sqlmock.NewResult(0, 0),
			assertResult: func(t *testing.T, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "a currency still referenced by an account is a foreign key violation",
			mockErr:      sqliteForeignKeyErr(),
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

			exp := mock.ExpectExec(deleteCurrencyQuery).WithArgs(code)
			if tt.mockErr != nil {
				exp.WillReturnError(tt.mockErr)
			} else {
				exp.WillReturnResult(tt.res)
			}

			found, err := s.DeleteCurrency(context.Background(), code)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
