package session

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/repository/session/mocks"
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

// fakeID, fakeToken and fakeExpiresAt are the field-shaped random values the tests below bind into
// mock expectations and returned rows, so a test failure is never masked by two cases accidentally
// sharing a fixture value.
func fakeID() uint64           { return uint64(gofakeit.Number(1, 1_000_000)) }
func fakeToken() string        { return gofakeit.UUID() }
func fakeExpiresAt() time.Time { return gofakeit.Date().UTC() }

func fakeSessionEntity() entity.Session {
	return entity.Session{
		Token:     fakeToken(),
		UserID:    fakeID(),
		ExpiresAt: fakeExpiresAt(),
	}
}

// errStub is the sentinel a case uses when it only cares that an error travels through untouched.
var errStub = errors.New(gofakeit.Sentence())

// TestRepository_Create covers the entity -> row conversion.
func TestRepository_Create(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage, session entity.Session)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "converts the entity and stores it",
			mock: func(m *mocks.MockStorage, session entity.Session) {
				want := model.Session{
					Token:     session.Token,
					UserID:    session.UserID,
					ExpiresAt: session.ExpiresAt,
				}
				m.EXPECT().CreateSession(context.Background(), want).Return(nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage, session entity.Session) {
				m.EXPECT().CreateSession(context.Background(), gomock.Any()).Return(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			session := fakeSessionEntity()
			tt.mock(m, session)

			err := r.Create(context.Background(), session)
			tt.assertErr(t, err)
		})
	}
}

// TestRepository_FindByToken covers the lookup, including the token -> message-less NotFoundError
// translation.
func TestRepository_FindByToken(t *testing.T) {
	t.Parallel()

	token := fakeToken()

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage) entity.Session
		assertResult func(t *testing.T, want, got entity.Session)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "finds the session",
			mock: func(m *mocks.MockStorage) entity.Session {
				row := model.Session{Token: token, UserID: fakeID(), ExpiresAt: fakeExpiresAt()}
				m.EXPECT().FindSessionByToken(context.Background(), token).Return(row, nil)

				return row.ToEntity()
			},
			assertResult: func(t *testing.T, want, got entity.Session) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "an unknown token becomes a message-less NotFoundError",
			mock: func(m *mocks.MockStorage) entity.Session {
				m.EXPECT().FindSessionByToken(context.Background(), token).Return(model.Session{}, sql.ErrNoRows)

				return entity.Session{}
			},
			assertResult: func(t *testing.T, want, got entity.Session) {},
			assertErr: func(t *testing.T, err error) {
				var notFound *domainerror.NotFoundError
				require.ErrorAs(t, err, &notFound)
				require.Empty(t, notFound.Message)
			},
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) entity.Session {
				m.EXPECT().FindSessionByToken(context.Background(), token).Return(model.Session{}, errStub)

				return entity.Session{}
			},
			assertResult: func(t *testing.T, want, got entity.Session) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			want := tt.mock(m)

			got, err := r.FindByToken(context.Background(), token)
			tt.assertErr(t, err)
			tt.assertResult(t, want, got)
		})
	}
}

// TestRepository_Delete covers the plain delegation to storage.
func TestRepository_Delete(t *testing.T) {
	t.Parallel()

	token := fakeToken()

	tests := []struct {
		name      string
		mock      func(m *mocks.MockStorage)
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteSession(context.Background(), token).Return(nil)
			},
			assertErr: func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteSession(context.Background(), token).Return(errStub)
			},
			assertErr: func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			err := r.Delete(context.Background(), token)
			tt.assertErr(t, err)
		})
	}
}

// TestRepository_DeleteExpired covers the plain delegation to storage, including the deleted-rows
// count it hands back.
func TestRepository_DeleteExpired(t *testing.T) {
	t.Parallel()

	now := fakeExpiresAt()
	want := int64(gofakeit.Number(0, 500))

	tests := []struct {
		name         string
		mock         func(m *mocks.MockStorage)
		assertResult func(t *testing.T, got int64)
		assertErr    func(t *testing.T, err error)
	}{
		{
			name: "delegates to storage",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteExpiredSessions(context.Background(), now).Return(want, nil)
			},
			assertResult: func(t *testing.T, got int64) { require.Equal(t, want, got) },
			assertErr:    func(t *testing.T, err error) { require.NoError(t, err) },
		},
		{
			name: "a storage error is propagated",
			mock: func(m *mocks.MockStorage) {
				m.EXPECT().DeleteExpiredSessions(context.Background(), now).Return(int64(0), errStub)
			},
			assertResult: func(t *testing.T, got int64) {},
			assertErr:    func(t *testing.T, err error) { require.ErrorIs(t, err, errStub) },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			r, m := newRepo(t)
			tt.mock(m)

			got, err := r.DeleteExpired(context.Background(), now)
			tt.assertErr(t, err)
			tt.assertResult(t, got)
		})
	}
}
