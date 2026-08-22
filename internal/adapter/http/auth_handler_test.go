package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"
)

func TestHandleLogin(t *testing.T) {
	t.Parallel()

	t.Run("malformed body is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{not json`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("wrong password against an existing account is rejected with 401", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		deps.users.countFn = func(context.Context) (int, error) { return 1, nil }
		deps.users.findByUsernameFn = func(context.Context, string) (entity.User, error) {
			return entity.User{ID: 1, Username: "admin", PasswordHash: "hashed"}, nil
		}
		deps.hasher.compareFn = func(string, string) bool { return false }

		body := `{"username":"admin","password":"wrong"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("empty database auto-provisions and sets a session cookie", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		deps.users.countFn = func(context.Context) (int, error) { return 0, nil }
		deps.hasher.hashFn = func(password string) (string, error) { return "hashed-" + password, nil }
		deps.users.createFn = func(context.Context, port.UserCreateRequest) (uint64, error) { return 1, nil }
		deps.tokens.newTokenFn = func() (string, error) { return "tok-123", nil }
		deps.sessions.createFn = func(context.Context, entity.Session) error { return nil }

		body := `{"username":"admin","password":"correcthorse"}`
		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"admin"`)

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, sessionCookieName, cookies[0].Name)
		require.Equal(t, "tok-123", cookies[0].Value)
	})
}

func TestHandleMe(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns the current session's user", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 7, Username: "admin"})

		req := httptest.NewRequest(http.MethodGet, "/api/auth/me", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"id":7,"username":"admin","settings":{}}`, rec.Body.String())
	})
}

func TestHandleLogout(t *testing.T) {
	t.Parallel()

	t.Run("clears the session cookie and returns 204", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.sessions.deleteFn = func(_ context.Context, token string) error {
			require.Equal(t, testToken, token)
			return nil
		}

		req := httptest.NewRequest(http.MethodPost, "/api/auth/logout", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNoContent, rec.Code)

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		require.Equal(t, sessionCookieName, cookies[0].Name)
		require.Negative(t, cookies[0].MaxAge)
	})
}

func TestHandleUpdateCredentials(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodPatch, "/api/auth/credentials", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("incorrect current password is rejected with 401", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1, Username: "admin", PasswordHash: "hashed"})
		deps.hasher.compareFn = func(string, string) bool { return false }

		body := `{"currentPassword":"wrong"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/auth/credentials", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("valid password change returns 200", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1, Username: "admin", PasswordHash: "hashed"})
		deps.hasher.compareFn = func(string, string) bool { return true }
		deps.hasher.hashFn = func(password string) (string, error) { return "hashed-" + password, nil }
		deps.users.updatePasswordHashFn = func(context.Context, uint64, string) error { return nil }

		body := `{"currentPassword":"correcthorse","newPassword":"newpassword"}`
		req := httptest.NewRequest(http.MethodPatch, "/api/auth/credentials", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}
