package mysql

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

func fakeAccount() model.Account {
	return model.Account{
		ID:           fakeID(),
		Name:         fakeName(),
		Type:         uint8(entity.AccountTypeCash),
		CurrencyCode: fakeCode(),
		Balance:      fakeAmount(),
		Status:       uint8(entity.AccountStatusActive),
	}
}

func addAccountRow(rows *sqlmock.Rows, a model.Account) *sqlmock.Rows {
	return rows.AddRow(a.ID, a.Name, a.Type, a.CurrencyCode, a.Balance, a.Status)
}

// TestListAccounts covers the full listing: every row comes back scanned into model.Account.
func TestListAccounts(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + accountColumns + ` FROM accounts ORDER BY created_at`

	t.Run("an empty result yields no rows", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"}))

		got, err := s.ListAccounts(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("multiple rows come back in order", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		a1, a2 := fakeAccount(), fakeAccount()

		rows := sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"})
		addAccountRow(rows, a1)
		addAccountRow(rows, a2)
		mock.ExpectQuery(query).WillReturnRows(rows)

		got, err := s.ListAccounts(context.Background())
		require.NoError(t, err)
		require.Equal(t, []model.Account{a1, a2}, got)
	})

	t.Run("a driver error is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListAccounts(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindAccountByID covers the single-row lookup, and that a miss is reported as sql.ErrNoRows.
func TestFindAccountByID(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + accountColumns + ` FROM accounts WHERE id = ?`
	a := fakeAccount()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Account)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the row",
			mock: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"})
				mock.ExpectQuery(query).WithArgs(a.ID).WillReturnRows(addAccountRow(rows, a))
			},
			assertResult: func(t *testing.T, got model.Account) { require.Equal(t, a, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "an unknown id is sql.ErrNoRows",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(a.ID).WillReturnError(sql.ErrNoRows) },
			assertResult: func(t *testing.T, got model.Account) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, sql.ErrNoRows) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.FindAccountByID(context.Background(), a.ID)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestCreateAccount covers the insert and the one error the balance column can produce: a negative
// opening balance, which MySQL's unsigned accounts.balance column reports as
// ER_WARN_DATA_OUT_OF_RANGE (a negative literal assignment — see storage.go).
func TestCreateAccount(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO accounts (name, type, currency, balance) VALUES (?, ?, ?, ?)`
	req := model.AccountCreateRequest{
		Name: fakeName(), Type: entity.AccountTypeCash, CurrencyCode: fakeCode(), Balance: fakeAmount(),
	}
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "creates the account",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
					WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a negative opening balance is an insufficient balance error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
					WillReturnError(mysqlErr(errWarnDataOutOfRange))
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.InsufficientBalanceError)
			},
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, req.Balance).
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

			got, err := s.CreateAccount(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestUpdateAccount covers the rename/re-type/archive path, which re-reads the row via
// FindAccountByID after a successful write, and the not-found case, which skips that re-read.
func TestUpdateAccount(t *testing.T) {
	t.Parallel()

	updateQuery := `UPDATE accounts SET name = ?, type = ?, currency = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`
	selectQuery := `SELECT ` + accountColumns + ` FROM accounts WHERE id = ?`

	req := model.AccountUpdateRequest{
		ID: fakeID(), Name: fakeName(), Type: entity.AccountTypeCard, CurrencyCode: fakeCode(), Archived: true,
	}
	wantRow := fakeAccount()
	wantRow.ID = req.ID

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
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusArchived), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 1))

				rows := sqlmock.NewRows([]string{"id", "name", "type", "currency", "balance", "status"})
				mock.ExpectQuery(selectQuery).WithArgs(req.ID).WillReturnRows(addAccountRow(rows, wantRow))
			},
			assertResult: func(t *testing.T, got model.Account, found bool) {
				require.True(t, found)
				require.Equal(t, wantRow, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is reported as not found, without a re-read",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusArchived), req.ID).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertResult: func(t *testing.T, got model.Account, found bool) { require.False(t, found) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error on the update is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(updateQuery).
					WithArgs(req.Name, uint8(req.Type), req.CurrencyCode, uint8(entity.AccountStatusArchived), req.ID).
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
				mock.ExpectExec(query).WithArgs(id).WillReturnError(mysqlErr(errRowIsReferenced))
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
