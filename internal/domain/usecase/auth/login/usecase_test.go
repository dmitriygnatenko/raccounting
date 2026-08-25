package login

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

// fakeUserRepository is a hand-rolled test double for port.UserRepository: each method call is
// routed to the matching function field, left nil (and so panicking on an unexpected call) unless a
// test case stubs it.
type fakeUserRepository struct {
	findByUsernameFn func(ctx context.Context, username string) (entity.User, error)
	findByIDFn       func(ctx context.Context, id uint64) (entity.User, error)
	createFn         func(ctx context.Context, req port.UserCreateRequest) (uint64, error)
	updateSettingsFn func(ctx context.Context, id uint64, settings entity.UserSettings) error
	countFn          func(ctx context.Context) (int, error)
}

func (f *fakeUserRepository) FindByUsername(ctx context.Context, username string) (entity.User, error) {
	return f.findByUsernameFn(ctx, username)
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id uint64) (entity.User, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeUserRepository) Create(ctx context.Context, req port.UserCreateRequest) (uint64, error) {
	return f.createFn(ctx, req)
}

func (f *fakeUserRepository) UpdateUsername(context.Context, uint64, string) error {
	panic("not stubbed")
}

func (f *fakeUserRepository) UpdatePasswordHash(context.Context, uint64, string) error {
	panic("not stubbed")
}

func (f *fakeUserRepository) GetSettings(context.Context, uint64) (entity.UserSettings, error) {
	panic("not stubbed")
}

func (f *fakeUserRepository) UpdateSettings(ctx context.Context, id uint64, settings entity.UserSettings) error {
	return f.updateSettingsFn(ctx, id, settings)
}

func (f *fakeUserRepository) Count(ctx context.Context) (int, error) {
	return f.countFn(ctx)
}

// fakeSessionRepository is a hand-rolled test double for port.SessionRepository.
type fakeSessionRepository struct {
	createFn func(ctx context.Context, session entity.Session) error
}

func (f *fakeSessionRepository) Create(ctx context.Context, session entity.Session) error {
	return f.createFn(ctx, session)
}

func (f *fakeSessionRepository) FindByToken(context.Context, string) (entity.Session, error) {
	panic("not stubbed")
}

func (f *fakeSessionRepository) Delete(context.Context, string) error {
	panic("not stubbed")
}

func (f *fakeSessionRepository) DeleteExpired(context.Context, time.Time) (int64, error) {
	panic("not stubbed")
}

// fakePasswordHasher is a hand-rolled test double for port.PasswordHasher.
type fakePasswordHasher struct {
	hashFn    func(password string) (string, error)
	compareFn func(hash, password string) bool
}

func (f *fakePasswordHasher) Hash(password string) (string, error) {
	return f.hashFn(password)
}

func (f *fakePasswordHasher) Compare(hash, password string) bool {
	return f.compareFn(hash, password)
}

// fakeTokenGenerator is a hand-rolled test double for port.TokenGenerator.
type fakeTokenGenerator struct {
	newTokenFn func() (string, error)
}

func (f *fakeTokenGenerator) NewToken() (string, error) {
	return f.newTokenFn()
}

// deps bundles the fakes a UseCase depends on.
type deps struct {
	user    *fakeUserRepository
	session *fakeSessionRepository
	hasher  *fakePasswordHasher
	token   *fakeTokenGenerator
}

// newUseCase returns a UseCase wired to fresh, unstubbed fakes; a test case stubs only the methods
// its scenario needs.
func newUseCase() (*UseCase, *deps) {
	d := &deps{
		user:    &fakeUserRepository{},
		session: &fakeSessionRepository{},
		hasher:  &fakePasswordHasher{},
		token:   &fakeTokenGenerator{},
	}

	return New(d.user, d.session, d.hasher, d.token), d
}

var errStub = errors.New("stub failure")

func assertIncorrectCredentials(t *testing.T, err error) {
	t.Helper()

	var unauthorized *domainerror.UnauthorizedError

	require.ErrorAs(t, err, &unauthorized)
	require.Equal(t, "Incorrect username or password", unauthorized.Message)
}

func TestUseCase_Execute(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		mock         func(d *deps) Input
		assertResult func(t *testing.T, got Output)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "empty username fails validation before any lookup",
			mock: func(d *deps) Input {
				return Input{Username: "", Password: "correcthorse"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr:    func(t *testing.T, err error) { require.EqualError(t, err, "Please enter a username") },
		},
		{
			name: "empty password fails validation before any lookup",
			mock: func(d *deps) Input {
				return Input{Username: "admin", Password: ""}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr:    func(t *testing.T, err error) { require.EqualError(t, err, "Please enter a password") },
		},
		{
			name: "empty database auto-provisions the account and starts a session",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 0, nil }
				d.hasher.hashFn = func(password string) (string, error) {
					require.Equal(t, "correcthorse", password)
					return "hashed-correcthorse", nil
				}
				d.user.createFn = func(_ context.Context, req port.UserCreateRequest) (uint64, error) {
					require.Equal(t, "admin", req.Username)
					require.Equal(t, "hashed-correcthorse", req.PasswordHash)
					return 1, nil
				}
				d.token.newTokenFn = func() (string, error) { return "tok-123", nil }
				d.session.createFn = func(_ context.Context, s entity.Session) error {
					require.Equal(t, "tok-123", s.Token)
					require.Equal(t, uint64(1), s.UserID)
					require.WithinDuration(t, time.Now().UTC().Add(sessionDuration), s.ExpiresAt, time.Minute)
					return nil
				}

				return Input{Username: "  Admin  ", Password: "correcthorse"}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, uint64(1), got.User.ID)
				require.Equal(t, "admin", got.User.Username)
				require.Equal(t, "tok-123", got.Session.Token)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "non-empty database rejects an unknown username",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 1, nil }
				d.user.findByUsernameFn = func(context.Context, string) (entity.User, error) {
					return entity.User{}, &domainerror.NotFoundError{Message: "not found"}
				}

				return Input{Username: "nobody", Password: "correcthorse"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr:    assertIncorrectCredentials,
		},
		{
			name: "non-empty database rejects a wrong password",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 1, nil }
				d.user.findByUsernameFn = func(context.Context, string) (entity.User, error) {
					return entity.User{ID: 1, Username: "admin", PasswordHash: "hashed"}, nil
				}
				d.hasher.compareFn = func(hash, password string) bool {
					require.Equal(t, "hashed", hash)
					require.Equal(t, "wrongpass", password)
					return false
				}

				return Input{Username: "admin", Password: "wrongpass"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr:    assertIncorrectCredentials,
		},
		{
			name: "correct credentials against an existing account start a session",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 1, nil }
				d.user.findByUsernameFn = func(context.Context, string) (entity.User, error) {
					return entity.User{ID: 7, Username: "admin", PasswordHash: "hashed"}, nil
				}
				d.hasher.compareFn = func(string, string) bool { return true }
				d.token.newTokenFn = func() (string, error) { return "tok-xyz", nil }
				d.session.createFn = func(_ context.Context, s entity.Session) error {
					require.Equal(t, uint64(7), s.UserID)
					return nil
				}

				return Input{Username: "admin", Password: "correcthorse"}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, uint64(7), got.User.ID)
				require.Equal(t, "tok-xyz", got.Session.Token)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "language is saved for an existing account that has none yet",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 1, nil }
				d.user.findByUsernameFn = func(context.Context, string) (entity.User, error) {
					return entity.User{ID: 7, Username: "admin", PasswordHash: "hashed"}, nil
				}
				d.hasher.compareFn = func(string, string) bool { return true }
				d.user.updateSettingsFn = func(_ context.Context, id uint64, settings entity.UserSettings) error {
					require.Equal(t, uint64(7), id)
					require.Equal(t, "fr", settings.Language)
					return nil
				}
				d.token.newTokenFn = func() (string, error) { return "tok-xyz", nil }
				d.session.createFn = func(context.Context, entity.Session) error { return nil }

				return Input{Username: "admin", Password: "correcthorse", Language: "fr"}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, "fr", got.User.Settings.Language)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an already-set language is not overwritten by the login form's language",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 1, nil }
				d.user.findByUsernameFn = func(context.Context, string) (entity.User, error) {
					return entity.User{
						ID: 7, Username: "admin", PasswordHash: "hashed",
						Settings: entity.UserSettings{Language: "ru"},
					}, nil
				}
				d.hasher.compareFn = func(string, string) bool { return true }
				d.user.updateSettingsFn = func(context.Context, uint64, entity.UserSettings) error {
					t.Fatal("UpdateSettings should not be called when a language is already saved")
					return nil
				}
				d.token.newTokenFn = func() (string, error) { return "tok-xyz", nil }
				d.session.createFn = func(context.Context, entity.Session) error { return nil }

				return Input{Username: "admin", Password: "correcthorse", Language: "en"}
			},
			assertResult: func(t *testing.T, got Output) {
				require.Equal(t, "ru", got.User.Settings.Language)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "token generation failure is reported",
			mock: func(d *deps) Input {
				d.user.countFn = func(context.Context) (int, error) { return 1, nil }
				d.user.findByUsernameFn = func(context.Context, string) (entity.User, error) {
					return entity.User{ID: 1, Username: "admin", PasswordHash: "hashed"}, nil
				}
				d.hasher.compareFn = func(string, string) bool { return true }
				d.token.newTokenFn = func() (string, error) { return "", errStub }

				return Input{Username: "admin", Password: "correcthorse"}
			},
			assertResult: func(t *testing.T, got Output) { require.Equal(t, Output{}, got) },
			assertErr: func(t *testing.T, err error) {
				require.EqualError(t, err, "Failed to generate a token")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			uc, d := newUseCase()
			input := tt.mock(d)

			got, err := uc.Execute(context.Background(), input)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
