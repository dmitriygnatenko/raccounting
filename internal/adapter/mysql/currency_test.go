package mysql

import (
	"context"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

func fakeCurrency() model.Currency {
	return model.Currency{
		Code: fakeCode(), Symbol: fakeSymbol(), Name: fakeName(), Rate: fakeRate(),
		Default: false, Status: uint8(entity.AccountStatusActive),
	}
}

func addCurrencyRow(rows *sqlmock.Rows, c model.Currency) *sqlmock.Rows {
	return rows.AddRow(c.Code, c.Symbol, c.Name, c.Rate, c.Default, c.Status)
}

// TestListCurrencies covers the full listing.
func TestListCurrencies(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + currencyColumns + ` FROM currencies ORDER BY created_at`

	t.Run("an empty result yields no rows", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(
			sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"}),
		)

		got, err := s.ListCurrencies(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("multiple rows come back in order", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		c1, c2 := fakeCurrency(), fakeCurrency()

		rows := sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"})
		addCurrencyRow(rows, c1)
		addCurrencyRow(rows, c2)
		mock.ExpectQuery(query).WillReturnRows(rows)

		got, err := s.ListCurrencies(context.Background())
		require.NoError(t, err)
		require.Equal(t, []model.Currency{c1, c2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListCurrencies(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestExistsCurrency covers the existence check currency validation relies on.
func TestExistsCurrency(t *testing.T) {
	t.Parallel()

	query := `SELECT COUNT(*) FROM currencies WHERE code = ?`
	code := fakeCode()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "an existing currency reports true",
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
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(code).WillReturnError(errStub)
			},
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

// TestCreateCurrency covers both branches: a non-default currency inserts directly, while a default
// one first clears every other row's is_default flag inside a DB transaction, atomically.
func TestCreateCurrency(t *testing.T) {
	t.Parallel()

	insertQuery := `INSERT INTO currencies (code, symbol, name, rate, is_default) VALUES (?, ?, ?, ?, ?)`
	clearDefaultQuery := `UPDATE currencies SET is_default = 0`

	req := model.CurrencyCreateRequest{Code: fakeCode(), Symbol: fakeSymbol(), Name: fakeName(), Rate: fakeRate()}

	t.Run("a non-default currency inserts directly, no transaction", func(t *testing.T) {
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
			WillReturnError(mysqlErr(errDuplicateEntry))

		err := s.CreateCurrency(context.Background(), req)
		require.ErrorIs(t, err, storageError.UniqueViolationError)
	})

	defaultReq := req
	defaultReq.Default = true

	t.Run("a default currency clears every other default first, atomically", func(t *testing.T) {
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

	t.Run("a taken code inside the default transaction rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(insertQuery).WithArgs(defaultReq.Code, defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true).
			WillReturnError(mysqlErr(errDuplicateEntry))
		mock.ExpectRollback()

		err := s.CreateCurrency(context.Background(), defaultReq)
		require.ErrorIs(t, err, storageError.UniqueViolationError)
	})

	t.Run("a driver error clearing the default flag rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnError(errStub)
		mock.ExpectRollback()

		err := s.CreateCurrency(context.Background(), defaultReq)
		require.ErrorIs(t, err, errStub)
	})
}

// TestUpdateCurrency covers both branches: a non-default update runs directly, while a default one
// runs inside a DB transaction that first clears every other row's is_default flag.
func TestUpdateCurrency(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE currencies SET symbol = ?, name = ?, rate = ?, is_default = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE code = ?`
	selectQuery := `SELECT ` + currencyColumns + ` FROM currencies WHERE code = ?`
	clearDefaultQuery := `UPDATE currencies SET is_default = 0`

	req := model.CurrencyUpdateRequest{
		Code: fakeCode(), Symbol: fakeSymbol(), Name: fakeName(), Rate: fakeRate(), Archived: false,
	}
	wantRow := fakeCurrency()
	wantRow.Code = req.Code

	t.Run("a non-default update re-reads the row, no transaction", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).
			WithArgs(req.Symbol, req.Name, req.Rate, false, uint8(entity.AccountStatusActive), req.Code).
			WillReturnResult(sqlmock.NewResult(0, 1))

		rows := sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"})
		mock.ExpectQuery(selectQuery).WithArgs(req.Code).WillReturnRows(addCurrencyRow(rows, wantRow))

		got, found, err := s.UpdateCurrency(context.Background(), req)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, wantRow, got)
	})

	t.Run("an unknown code is reported as not found, without a re-read", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).
			WithArgs(req.Symbol, req.Name, req.Rate, false, uint8(entity.AccountStatusActive), req.Code).
			WillReturnResult(sqlmock.NewResult(0, 0))

		_, found, err := s.UpdateCurrency(context.Background(), req)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the update is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectExec(updateQuery).
			WithArgs(req.Symbol, req.Name, req.Rate, false, uint8(entity.AccountStatusActive), req.Code).
			WillReturnError(errStub)

		_, found, err := s.UpdateCurrency(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})

	defaultReq := req
	defaultReq.Default = true

	t.Run("a default update clears every other default first, atomically", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(updateQuery).
			WithArgs(defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true, uint8(entity.AccountStatusActive), defaultReq.Code).
			WillReturnResult(sqlmock.NewResult(0, 1))

		rows := sqlmock.NewRows([]string{"code", "symbol", "name", "rate", "is_default", "status"})
		defaultWantRow := wantRow
		defaultWantRow.Default = true
		mock.ExpectQuery(selectQuery).WithArgs(defaultReq.Code).WillReturnRows(addCurrencyRow(rows, defaultWantRow))
		mock.ExpectCommit()

		got, found, err := s.UpdateCurrency(context.Background(), defaultReq)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, defaultWantRow, got)
	})

	t.Run("an unknown code inside the default transaction rolls back without commit", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(clearDefaultQuery).WillReturnResult(sqlmock.NewResult(0, 3))
		mock.ExpectExec(updateQuery).
			WithArgs(defaultReq.Symbol, defaultReq.Name, defaultReq.Rate, true, uint8(entity.AccountStatusActive), defaultReq.Code).
			WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectRollback()

		_, found, err := s.UpdateCurrency(context.Background(), defaultReq)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error clearing the default flag rolls back", func(t *testing.T) {
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

// TestDeleteCurrency covers the delete and the constraint failure a currency still referenced by an
// account produces.
func TestDeleteCurrency(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM currencies WHERE code = ?`
	code := fakeCode()

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
			name: "a currency still referenced by an account surfaces a foreign key violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(code).WillReturnError(mysqlErr(errRowIsReferenced))
			},
			assertResult: func(t *testing.T, found bool) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.ForeignKeyViolationError)
			},
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(code).WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, found bool) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
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
