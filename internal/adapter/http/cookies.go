package http

import (
	"net/http"
	"time"

	"raccounting/internal/domain/entity"
)

const sessionCookieName = "session"

// setSessionCookie writes the httpOnly session cookie for a freshly created session.
func (s *Server) setSessionCookie(w http.ResponseWriter, session entity.Session) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    session.Token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.CookieSecure,
		Expires:  session.ExpiresAt,
		MaxAge:   int(time.Until(session.ExpiresAt).Seconds()),
	})
}

// clearSessionCookie erases the session cookie in the browser.
func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   s.CookieSecure,
		MaxAge:   -1,
	})
}
