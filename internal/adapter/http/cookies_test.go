package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
)

func TestSetSessionCookie(t *testing.T) {
	t.Parallel()

	t.Run("insecure server writes a non-secure cookie", func(t *testing.T) {
		t.Parallel()

		s := &Server{CookieSecure: false}
		rec := httptest.NewRecorder()
		expiresAt := time.Now().UTC().Add(2 * time.Hour)

		s.setSessionCookie(rec, entity.Session{Token: "tok-123", ExpiresAt: expiresAt})

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)

		c := cookies[0]
		require.Equal(t, sessionCookieName, c.Name)
		require.Equal(t, "tok-123", c.Value)
		require.Equal(t, "/", c.Path)
		require.True(t, c.HttpOnly)
		require.False(t, c.Secure)
		require.Equal(t, http.SameSiteLaxMode, c.SameSite)
		require.WithinDuration(t, expiresAt, c.Expires, time.Second)
	})

	t.Run("secure server writes a secure cookie", func(t *testing.T) {
		t.Parallel()

		s := &Server{CookieSecure: true}
		rec := httptest.NewRecorder()

		s.setSessionCookie(rec, entity.Session{Token: "tok-456", ExpiresAt: time.Now().UTC().Add(time.Hour)})

		cookies := rec.Result().Cookies()
		require.Len(t, cookies, 1)
		require.True(t, cookies[0].Secure)
	})
}

func TestClearSessionCookie(t *testing.T) {
	t.Parallel()

	s := &Server{CookieSecure: false}
	rec := httptest.NewRecorder()

	s.clearSessionCookie(rec)

	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)

	c := cookies[0]
	require.Equal(t, sessionCookieName, c.Name)
	require.Equal(t, "", c.Value)
	require.Negative(t, c.MaxAge)
}
