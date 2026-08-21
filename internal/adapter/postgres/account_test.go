package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const accountSelectColumns = `id, name, type, currency, balance, status`

// fakeAccountType and fakeAccountStatus return a random known enum value, using the package's own
// enumerators (entity.AccountTypes) rather than a hand-picked constant, so the fixture stays valid if
// the set of types ever changes.
func fakeAccountType() entity.AccountType {
	types := entity.AccountTypes()
	return types[gofakeit.Number(0, len(types)-1)]
}

func fakeAccountRow() model.Account {
	return model.Account{
		ID:           fakeID(),
		Name:         fakeName(),
		Type:         uint8(fakeAccountType()),
		CurrencyCode: fakeCurrencyCode(),
		Balance:      fakeAmount(),
		Status:       uint8(entity.AccountStatusActive),
	}
}

func accountRowValues(m model.Account) []driver.Value {
	return []driver.Value{m.ID, m.Name, m.Type, m.CurrencyCode, m.Balance, m.Status}
}

// TestListAccounts covers the full listing: every row comes back scanned into model.Account, in
// whatever order the query returned them (ORDER BY is the query's job, not this method's).
func TestListAccounts(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + accountSelectColumns + ` FROM accounts ORDER BY created_at`

	one := fakeAccountRow()
	two := fakeAccountRow()

	tests := []struct {
		name string
		rows []model.Account
	}{
		{name: "an empty result yields no rows"},
		{name: "a single account", rows: []model.Account{one}},
		{name: "multiple accounts", rows: []model.Account{one, two}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"})
			for _, a := range tt.rows {
				mockRows.AddRow(accountRowValues(a)...)
			}

			mock.ExpectQuery(query).WillReturnRows(mockRows)

			got, err := s.ListAccounts(context.Background())
			require.NoError(t, err)
			require.Equal(t, tt.rows, got)
		})
	}

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListAccounts(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindAccountByID covers the single-row lookup every account read starts with.
func TestFindAccountByID(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + accountSelectColumns + ` FROM accounts WHERE id = $1`
	row := fakeAccountRow()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(row.ID).WillReturnRows(
					sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"}).
						AddRow(accountRowValues(row)...),
				)
			},
			assertResult: func(t *testing.T, got model.Account) { require.Equal(t, row, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is sql.ErrNoRows",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(row.ID).WillReturnError(sql.ErrNoRows)
			},
			assertResult: func(t *testing.T, got model.Account) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, sql.ErrNoRows) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.FindAccountByID(context.Background(), row.ID)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestCreateAccount covers the insert and the one error shape a negative opening balance produces —
// accounts.balance has CHECK (balance >= 0), and wrapInsufficientBalance is what turns that into
// storageError.InsufficientBalanceError.
func TestCreateAccount(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO accounts (name, type, currency, balance) VALUES ($1, $2, $3, $4) RETURNING id`

	req := model.AccountCreateRequest{
		Name: fakeName(), Type: fakeAccountType(), CurrencyCode: fakeCurrencyCode(), Balance: fakeAmount(),
	}
	wantID := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "creates the account",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, wantID, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a negative opening balance is an insufficient balance error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
					WillReturnError(balanceCheckErr())
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, storageError.InsufficientBalanceError) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.CreateAccount(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestUpdateAccount covers the update, its found/not-found translation, and the follow-up
// FindAccountByID read that produces the returned row.
func TestUpdateAccount(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE accounts SET name = $1, type = $2, currency = $3, status = $4, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $5`
	selectQuery := `SELECT ` + accountSelectColumns + ` FROM accounts WHERE id = $1`

	req := model.AccountUpdateRequest{
		ID: fakeID(), Name: fakeName(), Type: fakeAccountType(), CurrencyCode: fakeCurrencyCode(), Archived: false,
	}
	updated := fakeAccountRow()
	updated.ID = req.ID

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Account, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates and returns the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(
					sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"}).
						AddRow(accountRowValues(updated)...),
				)
			},
			assertResult: func(t *testing.T, got model.Account, found bool) {
				require.True(t, found)
				require.Equal(t, updated, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, got model.Account, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error on the update is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusActive), req.ID).
					WillReturnError(errStub)
			},
			assertResult: func(t *testing.T, got model.Account, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, found, err := s.UpdateAccount(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got, found)
		})
	}
}

// TestDeleteAccount covers the delete and the FOREIGN KEY violation a still-referenced account
// produces.
func TestDeleteAccount(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM accounts WHERE id = $1`
	id := fakeID()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "deletes an unreferenced account",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertResult: func(t *testing.T, found bool) { require.True(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an account still referenced by a transaction surfaces the driver's foreign key error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnError(pgErr(pgForeignKeyViolation))
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

			found, err := s.DeleteAccount(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
