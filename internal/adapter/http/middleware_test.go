package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase/auth/authenticate"
)

func TestWithCORS(t *testing.T) {
	t.Parallel()

	t.Run("OPTIONS preflight short-circuits with 204", func(t *testing.T) {
		t.Parallel()

		called := false
		next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true })
		handler := WithCORS(next)

		req := httptest.NewRequest(http.MethodOptions, "/api/accounts", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)
		require.False(t, called, "OPTIONS must not reach the wrapped handler")
		require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
	})

	t.Run("non-OPTIONS requests pass through with CORS headers set", func(t *testing.T) {
		t.Parallel()

		called := false
		next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})
		handler := WithCORS(next)

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		rec := httptest.NewRecorder()

		handler.ServeHTTP(rec, req)

		require.True(t, called)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "*", rec.Header().Get("Access-Control-Allow-Origin"))
		require.NotEmpty(t, rec.Header().Get("Access-Control-Allow-Methods"))
	})
}

func TestWithLogging(t *testing.T) {
	t.Parallel()

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusTeapot)
	})
	handler := WithLogging(next)

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	require.True(t, called)
	require.Equal(t, http.StatusTeapot, rec.Code)
}

func TestUserFromContext(t *testing.T) {
	t.Parallel()

	t.Run("no user attached returns nil", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		require.Nil(t, userFromContext(req))
	})

	t.Run("returns the user requireAuth attached", func(t *testing.T) {
		t.Parallel()

		user := &entity.PublicUser{ID: 7, Username: "admin"}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req = req.WithContext(context.WithValue(req.Context(), userContextKey, user))

		require.Equal(t, user, userFromContext(req))
	})
}

func TestSessionToken(t *testing.T) {
	t.Parallel()

	t.Run("no cookie returns empty string", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)

		require.Empty(t, sessionToken(req))
	})

	t.Run("cookie value is returned", func(t *testing.T) {
		t.Parallel()

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "tok-abc"})

		require.Equal(t, "tok-abc", sessionToken(req))
	})
}

func TestRequireAuth(t *testing.T) {
	t.Parallel()

	newAuthServer := func() (*Server, *fakeSessionRepository, *fakeUserRepository) {
		sessions := &fakeSessionRepository{}
		users := &fakeUserRepository{}

		s := &Server{Auth: AuthUseCases{Authenticate: authenticate.New(sessions, users)}}

		return s, sessions, users
	}

	t.Run("missing cookie is rejected before any lookup", func(t *testing.T) {
		t.Parallel()

		s, _, _ := newAuthServer()
		called := false
		handler := s.requireAuth(func(http.ResponseWriter, *http.Request) { called = true })

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		rec := httptest.NewRecorder()

		handler(rec, req)

		require.False(t, called)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("unknown token is rejected", func(t *testing.T) {
		t.Parallel()

		s, sessions, _ := newAuthServer()
		sessions.findByTokenFn = func(context.Context, string) (entity.Session, error) {
			return entity.Session{}, errors.New("stub failure")
		}

		called := false
		handler := s.requireAuth(func(http.ResponseWriter, *http.Request) { called = true })

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "bogus"})
		rec := httptest.NewRecorder()

		handler(rec, req)

		require.False(t, called)
		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("valid session attaches the user and calls next", func(t *testing.T) {
		t.Parallel()

		s, sessions, users := newAuthServer()
		sessions.findByTokenFn = func(context.Context, string) (entity.Session, error) {
			return entity.Session{Token: "good", UserID: 42, ExpiresAt: time.Now().UTC().Add(time.Hour)}, nil
		}
		users.findByIDFn = func(context.Context, uint64) (entity.User, error) {
			return entity.User{ID: 42, Username: "admin"}, nil
		}

		var gotUser *entity.PublicUser
		handler := s.requireAuth(func(w http.ResponseWriter, r *http.Request) {
			gotUser = userFromContext(r)
			w.WriteHeader(http.StatusOK)
		})

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "good"})
		rec := httptest.NewRecorder()

		handler(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.NotNil(t, gotUser)
		require.Equal(t, uint64(42), gotUser.ID)
	})
}
