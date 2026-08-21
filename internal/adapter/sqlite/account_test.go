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

const accountSelectQuery = `SELECT id, name, type, currency, balance, status FROM accounts`

var accountColumnNames = []string{"id", "name", "type", "currency", "balance", "status"}

// fakeAccount returns a random model.Account, as scanAccount would produce it (CreatedAt/UpdatedAt
// are left zero — accountColumns doesn't select them).
func fakeAccount() model.Account {
	return model.Account{
		ID:           fakeID(),
		Name:         fakeName(),
		Type:         uint8(entity.AccountTypeCash),
		CurrencyCode: fakeCurrencyCode(),
		Balance:      fakeAmount(),
		Status:       uint8(entity.AccountStatusActive),
	}
}

func addAccountRow(rows *sqlmock.Rows, m model.Account) *sqlmock.Rows {
	return rows.AddRow(m.ID, m.Name, m.Type, m.CurrencyCode, m.Balance, m.Status)
}

// TestListAccounts covers the full listing: every row comes back scanned into model.Account.
func TestListAccounts(t *testing.T) {
	t.Parallel()

	query := accountSelectQuery + ` ORDER BY created_at`

	a1, a2 := fakeAccount(), fakeAccount()

	tests := []struct {
		name string
		rows []model.Account
	}{
		{name: "an empty result yields no rows"},
		{name: "a single account", rows: []model.Account{a1}},
		{name: "several accounts", rows: []model.Account{a1, a2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			mockRows := sqlmock.NewRows(accountColumnNames)
			for _, a := range tt.rows {
				addAccountRow(mockRows, a)
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

// TestFindAccountByID covers the single-row lookup. sql.ErrNoRows if it doesn't exist.
func TestFindAccountByID(t *testing.T) {
	t.Parallel()

	query := accountSelectQuery + ` WHERE id = ?`
	want := fakeAccount()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(want.ID).WillReturnRows(
					addAccountRow(sqlmock.NewRows(accountColumnNames), want),
				)
			},
			assertResult: func(t *testing.T, got model.Account) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is sql.ErrNoRows",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(want.ID).WillReturnError(sql.ErrNoRows)
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

			got, err := s.FindAccountByID(context.Background(), want.ID)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestCreateAccount covers the insert and the one error shape the balance column can produce: a
// negative opening balance, which comes back wrapped in storageError.InsufficientBalanceError.
func TestCreateAccount(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO accounts (name, type, currency, balance) VALUES (?, ?, ?, ?)`

	req := model.AccountCreateRequest{
		Name:         fakeName(),
		Type:         entity.AccountTypeCash,
		CurrencyCode: fakeCurrencyCode(),
		Balance:      fakeAmount(),
	}
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "inserts the account",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
					WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a negative opening balance is an insufficient balance violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
					WillReturnError(sqliteInsufficientBalanceErr())
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.InsufficientBalanceError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			id, err := s.CreateAccount(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, id)
		})
	}
}

// TestUpdateAccount covers the update, which re-reads the full row on success, and the not-found
// case, where the follow-up read never happens.
func TestUpdateAccount(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE accounts SET name = ?, type = ?, currency = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`
	selectQuery := accountSelectQuery + ` WHERE id = ?`

	req := model.AccountUpdateRequest{
		ID:           fakeID(),
		Name:         fakeName(),
		Type:         entity.AccountTypeCash,
		CurrencyCode: fakeCurrencyCode(),
		Archived:     false,
	}
	want := fakeAccount()
	want.ID = req.ID

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Account, found bool)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "updates and re-reads the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusActive), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 1))
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(
					addAccountRow(sqlmock.NewRows(accountColumnNames), want),
				)
			},
			assertResult: func(t *testing.T, got model.Account, found bool) {
				require.True(t, found)
				require.Equal(t, want, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found, no re-read",
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

// TestDeleteAccount covers the delete and the constraint failure an account still referenced by a
// transaction produces.
func TestDeleteAccount(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM accounts WHERE id = ?`
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
			name: "an account still referenced by a transaction surfaces a foreign key violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnError(sqliteForeignKeyErr())
			},
			assertResult: func(t *testing.T, found bool) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.ForeignKeyViolationError)
			},
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(id).WillReturnError(errStub)
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

			found, err := s.DeleteAccount(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, found)
		})
	}
}
