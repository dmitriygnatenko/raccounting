package sqlite

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

// fakeUserSettings returns a random settings value — the model.UserSettings this package's queries
// bind and scan through the settings JSON column.
func fakeUserSettings() model.UserSettings {
	return model.UserSettings{Language: gofakeit.LanguageAbbreviation()}
}

// mustSettingsJSON encodes v the way model.UserSettings.Value does, for building the exact
// driver.Value a mocked expectation has to match. A plain UserSettings struct can't fail to marshal.
func mustSettingsJSON(v model.UserSettings) string {
	b, err := v.Value()
	if err != nil {
		panic(err)
	}

	return b.(string)
}

// TestFindUserByUsername covers the login lookup: the row comes back whole, and a miss is reported as
// sql.ErrNoRows rather than a zero-valued user.
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
					sqlmock.NewRows([]string{
						"id",
						"username",
						"password_hash",
						"settings",
					}).
						AddRow(id, username, hash, mustSettingsJSON(settings)),
				)
			},
			assertResult: func(t *testing.T, got model.User) {
				require.Equal(t, model.User{
					ID:           id,
					Username:     username,
					PasswordHash: hash,
					Settings:     settings,
				}, got)
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
					sqlmock.NewRows([]string{
						"id",
						"username",
						"password_hash",
						"settings",
					}).
						AddRow(id, username, hash, mustSettingsJSON(settings)),
				)
			},
			assertResult: func(t *testing.T, got model.User) {
				require.Equal(t, model.User{
					ID:           id,
					Username:     username,
					PasswordHash: hash,
					Settings:     settings,
				}, got)
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
				mock.ExpectExec(query).WithArgs(username, hash).
					WillReturnResult(sqlmock.NewResult(wantID, 1))
			},
			assertResult: func(t *testing.T, got uint64) { require.Equal(t, uint64(wantID), got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken username is a unique violation",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(query).WithArgs(username, hash).
					WillReturnError(sqliteUniqueErr())
			},
			assertResult: func(t *testing.T, got uint64) {},
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

			id, err := s.CreateUser(context.Background(), username, hash)
			tt.assertErr(t, err)
			tt.assertResult(t, id)
		})
	}
}

// TestUpdateUsername covers the rename: a taken username has to come back as
// storageError.UniqueViolationError.
func TestUpdateUsername(t *testing.T) {
	t.Parallel()

	query := `UPDATE users SET username = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	id := fakeID()
	newUsername := fakeUsername()

	tests := []struct {
		name      string
		res       sql.Result
		mockErr   error
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "renames the user",
			res:       sqlmock.NewResult(0, 1),
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:      "an unknown id is not an error",
			res:       sqlmock.NewResult(0, 0),
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:    "a taken username is a unique violation",
			mockErr: sqliteUniqueErr(),
			assertErr: func(t *testing.T, err error) {
				require.ErrorIs(t, err, storageError.UniqueViolationError)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			s, mock := newMock(t)

			exp := mock.ExpectExec(query).WithArgs(newUsername, id)
			if tt.mockErr != nil {
				exp.WillReturnError(tt.mockErr)
			} else {
				exp.WillReturnResult(tt.res)
			}

			err := s.UpdateUsername(context.Background(), id, newUsername)
			tt.assertErr(t, err)
		})
	}
}

// TestUpdateUserPasswordHash covers the password change.
func TestUpdateUserPasswordHash(t *testing.T) {
	t.Parallel()

	query := `UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	id := fakeID()
	hash := fakeHash()

	tests := []struct {
		name      string
		res       sql.Result
		mockErr   error
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "overwrites the stored hash",
			res:       sqlmock.NewResult(0, 1),
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name:      "an unknown id changes nothing, not an error",
			res:       sqlmock.NewResult(0, 0),
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
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

			exp := mock.ExpectExec(query).WithArgs(hash, id)
			if tt.mockErr != nil {
				exp.WillReturnError(tt.mockErr)
			} else {
				exp.WillReturnResult(tt.res)
			}

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

// TestGetUserSettings covers reading the settings JSON column, including the NULL case (a user row
// with no settings yet decodes to the zero value rather than erroring).
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
			name: "returns the stored settings",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(userID).
					WillReturnRows(sqlmock.NewRows([]string{"settings"}).AddRow(mustSettingsJSON(settings)))
			},
			assertResult: func(t *testing.T, got model.UserSettings) { require.Equal(t, settings, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a NULL column decodes to the zero value",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(userID).
					WillReturnRows(sqlmock.NewRows([]string{"settings"}).AddRow(nil))
			},
			assertResult: func(t *testing.T, got model.UserSettings) { require.Equal(t, model.UserSettings{}, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id is sql.ErrNoRows",
			mock: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(query).WithArgs(userID).WillReturnError(sql.ErrNoRows)
			},
			assertResult: func(t *testing.T, got model.UserSettings) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, sql.ErrNoRows) },
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

// TestUpdateUserSettings covers overwriting the settings JSON blob wholesale.
func TestUpdateUserSettings(t *testing.T) {
	t.Parallel()

	query := `UPDATE users SET settings = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	userID := fakeID()
	settings := fakeUserSettings()

	tests := []struct {
		name      string
		mockErr   error
		assertErr func(t *testing.T, err error)
	}{
		{
			name:      "stores the settings",
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
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

			exp := mock.ExpectExec(query).WithArgs(mustSettingsJSON(settings), userID)
			if tt.mockErr != nil {
				exp.WillReturnError(tt.mockErr)
			} else {
				exp.WillReturnResult(sqlmock.NewResult(0, 1))
			}

			err := s.UpdateUserSettings(context.Background(), userID, settings)
			tt.assertErr(t, err)
		})
	}
}
