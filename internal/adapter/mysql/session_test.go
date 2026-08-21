package mysql

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	"raccounting/internal/storage/model"
)

// TestCreateSession covers the insert behind every login. Unlike most of this package's writes,
// CreateSession returns whatever the driver reports untouched — there's no wrapUnique/wrapForeignKey
// call here — so a constraint violation (an unknown user, a token already in use) surfaces as a
// plain *mysqldriver.MySQLError, not one of the storage package's sentinels.
func TestCreateSession(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO sessions (token, user_id, expires_at) VALUES (?, ?, ?)`

	token := fakeToken()
	userID := fakeID()
	expiresAt := fakeTime()

	tests := []struct {
		name      string
		mock      func(mock sqlmock.Sqlmock)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "stores the session",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(token, userID, expiresAt).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown user surfaces the driver's foreign key error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(token, userID, expiresAt).WillReturnError(mysqlErr(errRowIsReferenced))
			},
			assertErr: func(t *testing.T, err error) { require.Error(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(token, userID, expiresAt).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			session := model.Session{Token: token, UserID: userID, ExpiresAt: expiresAt}

			err := s.CreateSession(context.Background(), session)
			tt.assertErr(t, err)
		})
	}
}

// TestFindSessionByToken covers the lookup every authenticated request starts with. The expiry has
// to survive the round-trip intact — the session is checked against it, not against a stored flag.
func TestFindSessionByToken(t *testing.T) {
	t.Parallel()

	query := `SELECT user_id, expires_at FROM sessions WHERE token = ?`
	token := fakeToken()
	userID := fakeID()
	expiresAt := fakeTime()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.Session)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the session",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(token).WillReturnRows(
					sqlmock.NewRows([]string{"user_id", "expires_at"}).AddRow(userID, expiresAt),
				)
			},
			assertResult: func(t *testing.T, got model.Session) {
				require.Equal(t, token, got.Token)
				require.Equal(t, userID, got.UserID)
				require.True(t, got.ExpiresAt.Equal(expiresAt))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "an unknown token is sql.ErrNoRows",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(token).WillReturnError(sql.ErrNoRows) },
			assertResult: func(t *testing.T, got model.Session) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, sql.ErrNoRows) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(token).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got model.Session) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.FindSessionByToken(context.Background(), token)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestDeleteSession covers logout. Unlike most deletes in this package, it never calls affected —
// there's nothing distinguishing "removed" from "already gone" for the caller.
func TestDeleteSession(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM sessions WHERE token = ?`
	token := fakeToken()

	tests := []struct {
		name      string
		mock      func(mock sqlmock.Sqlmock)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "removes the session",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(token).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown token is not an error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(token).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(token).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			err := s.DeleteSession(context.Background(), token)
			tt.assertErr(t, err)
		})
	}
}

// TestDeleteExpiredSessions covers the cleanup sweep: the row count RowsAffected reports is what
// gets returned to the caller.
func TestDeleteExpiredSessions(t *testing.T) {
	t.Parallel()

	query := `DELETE FROM sessions WHERE expires_at < ?`
	now := fakeTime()
	wantDeleted := int64(gofakeit.Number(0, 100))

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got int64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "reports how many sessions were swept",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(now).WillReturnResult(sqlmock.NewResult(0, wantDeleted))
			},
			assertResult: func(t *testing.T, got int64) { require.Equal(t, wantDeleted, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectExec(query).WithArgs(now).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got int64) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.DeleteExpiredSessions(context.Background(), now)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
