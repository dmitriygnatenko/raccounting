package mysql

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"

	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

func fakeUserSettings() model.UserSettings {
	return model.UserSettings{Language: gofakeit.LanguageAbbreviation()}
}

// settingsValue is the raw driver.Value model.UserSettings.Value() produces — the form
// database/sql converts the struct into before a query arg or a mocked row ever reaches sqlmock.
func settingsValue(t *testing.T, s model.UserSettings) any {
	t.Helper()

	v, err := s.Value()
	require.NoError(t, err)

	return v
}

// TestFindUserByUsername covers the login lookup: the row comes back whole, and a miss is reported
// as sql.ErrNoRows rather than a zero-valued user.
func TestFindUserByUsername(t *testing.T) { //nolint:dupl // mirrors TestFindUserByID for a different query
	t.Parallel()

	query := `SELECT id, username, password_hash, settings FROM users WHERE username = ?`

	id := fakeID()
	username := fakeUsername()
	hash := fakeHash()
	settings := fakeUserSettings()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.User)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(username).WillReturnRows(
					sqlmock.NewRows([]string{"id", "username", "password_hash", "settings"}).
						AddRow(id, username, hash, settingsValue(t, settings)),
				)
			},
			assertResult: func(t *testing.T, got model.User) {
				require.Equal(t, model.User{ID: id, Username: username, PasswordHash: hash, Settings: settings}, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown username is sql.ErrNoRows",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(username).WillReturnError(sql.ErrNoRows)
			},
			assertResult: func(t *testing.T, got model.User) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, sql.ErrNoRows) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(username).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got model.User) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.FindUserByUsername(context.Background(), username)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestFindUserByID covers the same lookup by primary key, which is what every authenticated request
// goes through.
func TestFindUserByID(t *testing.T) { //nolint:dupl // mirrors TestFindUserByUsername for a different query
	t.Parallel()

	query := `SELECT id, username, password_hash, settings FROM users WHERE id = ?`

	id := fakeID()
	username := fakeUsername()
	hash := fakeHash()
	settings := fakeUserSettings()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.User)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the row",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(id).WillReturnRows(
					sqlmock.NewRows([]string{"id", "username", "password_hash", "settings"}).
						AddRow(id, username, hash, settingsValue(t, settings)),
				)
			},
			assertResult: func(t *testing.T, got model.User) {
				require.Equal(t, model.User{ID: id, Username: username, PasswordHash: hash, Settings: settings}, got)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "an unknown id is sql.ErrNoRows",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(id).WillReturnError(sql.ErrNoRows) },
			assertResult: func(t *testing.T, got model.User) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, sql.ErrNoRows) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(id).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got model.User) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.FindUserByID(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestCreateUser covers the insert and the one error the repositories act on: a taken username,
// which has to arrive as storageError.UniqueViolationError and not as a raw driver error.
func TestCreateUser(t *testing.T) {
	t.Parallel()

	query := `INSERT INTO users (username, password_hash) VALUES (?, ?)`

	username, hash := fakeUsername(), fakeHash()
	wantID := int64(fakeID())

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the credentials",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(username, hash).WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken username is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(username, hash).WillReturnError(mysqlErr(errDuplicateEntry))
			},
			assertResult: func(t *testing.T, got uint64) {},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.UniqueViolationError)
			},
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(username, hash).WillReturnError(errStub)
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

			id, err := s.CreateUser(context.Background(), username, hash)
			tt.assertErr(t, err)
			tt.assertResult(t, id)
		})
	}
}

// TestUpdateUsername covers the rename: a taken username has to come back as
// storageError.UniqueViolationError. Unlike account/category/tag, this never checks affected — an
// id that doesn't exist changes nothing and isn't reported either way.
func TestUpdateUsername(t *testing.T) {
	t.Parallel()

	query := `UPDATE users SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	id := fakeID()
	newUsername := fakeUsername()

	tests := []struct {
		name      string
		mock      func(mock sqlmock.Sqlmock)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "renames the user",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(newUsername, id).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is not an error",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(newUsername, id).WillReturnResult(sqlmock.NewResult(0, 0))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken username is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(newUsername, id).WillReturnError(mysqlErr(errDuplicateEntry))
			},
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.UniqueViolationError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			err := s.UpdateUsername(context.Background(), id, newUsername)
			tt.assertErr(t, err)
		})
	}
}

// TestUpdateUserPasswordHash covers the password change, with no constraint to wrap.
func TestUpdateUserPasswordHash(t *testing.T) {
	t.Parallel()

	query := `UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	id := fakeID()
	hash := fakeHash()

	tests := []struct {
		name      string
		mock      func(mock sqlmock.Sqlmock)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "overwrites the stored hash",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(hash, id).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(hash, id).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			err := s.UpdateUserPasswordHash(context.Background(), id, hash)
			tt.assertErr(t, err)
		})
	}
}

// TestCountUsers covers the count the first-run seeding decision is made on.
func TestCountUsers(t *testing.T) {
	t.Parallel()

	query := `SELECT COUNT(*) FROM users`
	want := gofakeit.Number(0, 1000)

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got int)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "counts the users",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(want))
			},
			assertResult: func(t *testing.T, got int) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got int) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.CountUsers(context.Background())
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestGetUserSettings covers the settings JSON column read, decoded through
// model.UserSettings.Scan.
func TestGetUserSettings(t *testing.T) {
	t.Parallel()

	query := `SELECT settings FROM users WHERE id = ?`
	userID := fakeID()
	settings := fakeUserSettings()

	tests := []struct {
		name         string
		mock         func(mock sqlmock.Sqlmock)
		assertResult func(t *testing.T, got model.UserSettings)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "decodes the stored settings",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(userID).
					WillReturnRows(sqlmock.NewRows([]string{"settings"}).AddRow(settingsValue(t, settings)))
			},
			assertResult: func(t *testing.T, got model.UserSettings) { require.Equal(t, settings, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a NULL column is the zero value",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(userID).
					WillReturnRows(sqlmock.NewRows([]string{"settings"}).AddRow(nil))
			},
			assertResult: func(t *testing.T, got model.UserSettings) { require.Equal(t, model.UserSettings{}, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:         "a driver error is propagated",
			mock:         func(mock sqlmock.Sqlmock) { mock.ExpectQuery(query).WithArgs(userID).WillReturnError(errStub) },
			assertResult: func(t *testing.T, got model.UserSettings) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			got, err := s.GetUserSettings(context.Background(), userID)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}

// TestUpdateUserSettings covers the settings JSON column overwrite, encoded through
// model.UserSettings.Value.
func TestUpdateUserSettings(t *testing.T) {
	t.Parallel()

	query := `UPDATE users SET settings = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	userID := fakeID()
	settings := fakeUserSettings()

	tests := []struct {
		name      string
		mock      func(mock sqlmock.Sqlmock)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "overwrites the stored settings",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(settingsValue(t, settings), userID).WillReturnResult(sqlmock.NewResult(0, 1))
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a driver error is propagated",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(settingsValue(t, settings), userID).WillReturnError(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)
			tt.mock(mock)

			err := s.UpdateUserSettings(context.Background(), userID, settings)
			tt.assertErr(t, err)
		})
	}
}
