package user

import (
	"context"
	"database/sql"
	"errors"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	"raccounting/internal/repository/user/mocks"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

// newRepo returns a Repository wired to a fresh MockStorage; any call a test doesn't stub via
// EXPECT() fails it, exactly like an unmet sqlmock expectation would in the adapter tests.
func newRepo(t *testing.T) (*Repository, *mocks.MockStorage) {
	t.Helper()

	mc := gomock.NewController(t)
	t.Cleanup(mc.Finish)

	m := mocks.NewMockStorage(mc)

	return New(m), m
}

// fakeID, fakeUsername, fakeHash and fakeLang are the field-shaped random values the tests below
// bind into mock expectations and returned rows, so a test failure is never masked by two cases
// accidentally sharing a fixture value.
func fakeID() uint64       { return uint64(gofakeit.Number(1, 1_000_000)) }
func fakeUsername() string { return gofakeit.Username() }
func fakeHash() string     { return gofakeit.LetterN(60) }
func fakeLang() string     { return gofakeit.LanguageAbbreviation() }

func fakeUserModel() model.User {
	return model.User{
		ID:           fakeID(),
		Username:     fakeUsername(),
		PasswordHash: fakeHash(),
		Settings:     model.UserSettings{Language: fakeLang()},
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_FindByUsername covers the lookup, including the username -> NotFoundError
// translation.
func TestRepository_FindByUsername(t *testing.T) { //nolint:dupl // mirrors TestRepository_FindByID
	t.Parallel()

	username := fakeUsername()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.User
		assertResult func(t *testing.T, want, got entity.User)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the user",
			mock: func(m *mocks.MockStorage) entity.User {
				u := fakeUserModel()
				u.Username = username
				m.EXPECT().FindUserByUsername(context.Background(), username).Return(u, nil)

				return u.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.User) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown username becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.User {
				m.EXPECT().FindUserByUsername(context.Background(), username).Return(model.User{}, sql.ErrNoRows)

				return entity.User{}
			},
			assertResult: func(t *testing.T, want, got entity.User) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.User {
				m.EXPECT().FindUserByUsername(context.Background(), username).Return(model.User{}, errStub)

				return entity.User{}
			},
			assertResult: func(t *testing.T, want, got entity.User) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.FindByUsername(context.Background(), username)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_FindByID covers the lookup, including the id -> NotFoundError translation.
func TestRepository_FindByID(t *testing.T) { //nolint:dupl // mirrors TestRepository_FindByUsername
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.User
		assertResult func(t *testing.T, want, got entity.User)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the user",
			mock: func(m *mocks.MockStorage) entity.User {
				u := fakeUserModel()
				u.ID = id
				m.EXPECT().FindUserByID(context.Background(), id).Return(u, nil)

				return u.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.User) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown id becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.User {
				m.EXPECT().FindUserByID(context.Background(), id).Return(model.User{}, sql.ErrNoRows)

				return entity.User{}
			},
			assertResult: func(t *testing.T, want, got entity.User) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.User {
				m.EXPECT().FindUserByID(context.Background(), id).Return(model.User{}, errStub)

				return entity.User{}
			},
			assertResult: func(t *testing.T, want, got entity.User) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.FindByID(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Create covers the insert and the username -> ConflictError translation, the one
// error shape this repository is allowed to recognize here.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	req := port.UserCreateRequest{
		Username:     fakeUsername(),
		PasswordHash: fakeHash(),
	}

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) uint64
		assertResult func(t *testing.T, wantID, got uint64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "stores the user and returns its id",
			mock: func(m *mocks.MockStorage) uint64 {
				wantID := fakeID()
				m.EXPECT().CreateUser(context.Background(), req.Username, req.PasswordHash).Return(wantID, nil)

				return wantID
			},
			assertResult: func(t *testing.T, wantID, got uint64) { require.Equal(t, wantID, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken username becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) uint64 {
				m.EXPECT().
					CreateUser(context.Background(), req.Username, req.PasswordHash).
					Return(uint64(0), storageError.UniqueViolationError)

				return 0
			},
			assertResult: func(t *testing.T, wantID, got uint64) {},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) uint64 {
				m.EXPECT().
					CreateUser(context.Background(), req.Username, req.PasswordHash).
					Return(uint64(0), errStub)

				return 0
			},
			assertResult: func(t *testing.T, wantID, got uint64) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			wantID := tt.mock(m)

			got, err := r.Create(context.Background(), req)
			tt.assertErr(t, err)
			tt.assertResult(t, wantID, got)
		})
	}
}

// TestRepository_UpdateUsername covers the rename and the username -> ConflictError translation.
func TestRepository_UpdateUsername(t *testing.T) {
	t.Parallel()

	id := fakeID()
	username := fakeUsername()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "renames the user",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().UpdateUsername(context.Background(), id, username).Return(nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a taken username becomes a message-less ConflictError",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().UpdateUsername(context.Background(), id, username).
					Return(storageError.UniqueViolationError)
			},
			assertErr: func(t *testing.T, err error) {
				var conflict *domainerror.ConflictError
				require.ErrorAs(t, err, &conflict)
				require.Empty(t, conflict.Message)
			},
		},
		{
			name: "any other storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().UpdateUsername(context.Background(), id, username).Return(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.UpdateUsername(context.Background(), id, username)
			tt.assertErr(t, err)
		})
	}
}

// TestRepository_UpdatePasswordHash covers the plain delegation to storage.
func TestRepository_UpdatePasswordHash(t *testing.T) { //nolint:dupl // mirrors sibling field-update tests
	t.Parallel()

	id := fakeID()
	hash := fakeHash()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().UpdateUserPasswordHash(context.Background(), id, hash).Return(nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().UpdateUserPasswordHash(context.Background(), id, hash).Return(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.UpdatePasswordHash(context.Background(), id, hash)
			tt.assertErr(t, err)
		})
	}
}

// TestRepository_GetSettings covers the row -> entity conversion and plain error propagation.
func TestRepository_GetSettings(t *testing.T) {
	t.Parallel()

	id := fakeID()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.UserSettings
		assertResult func(t *testing.T, want, got entity.UserSettings)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "returns the saved settings",
			mock: func(m *mocks.MockStorage) entity.UserSettings {
				settings := model.UserSettings{Language: fakeLang()}
				m.EXPECT().GetUserSettings(context.Background(), id).Return(settings, nil)

				return settings.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.UserSettings) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.UserSettings {
				m.EXPECT().GetUserSettings(context.Background(), id).Return(model.UserSettings{}, errStub)

				return entity.UserSettings{}
			},
			assertResult: func(t *testing.T, want, got entity.UserSettings) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.GetSettings(context.Background(), id)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_UpdateSettings covers wrapping the language into the storage settings shape.
func TestRepository_UpdateSettings(t *testing.T) {
	t.Parallel()

	id := fakeID()
	lang := fakeLang()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage with the language wrapped in settings",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().
					UpdateUserSettings(context.Background(), id, model.UserSettings{Language: lang}).
					Return(nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().
					UpdateUserSettings(context.Background(), id, model.UserSettings{Language: lang}).
					Return(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.UpdateSettings(context.Background(), id, lang)
			tt.assertErr(t, err)
		})
	}
}

// TestRepository_Count covers the plain delegation to storage.
func TestRepository_Count(t *testing.T) {
	t.Parallel()

	want := gofakeit.Number(0, 500)

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage)
		assertResult func(t *testing.T, got int)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().CountUsers(context.Background()).Return(want, nil)
			},
			assertResult: func(t *testing.T, got int) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().CountUsers(context.Background()).Return(0, errStub)
			},
			assertResult: func(t *testing.T, got int) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			got, err := r.Count(context.Background())
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
