package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase/auth/authenticate"
)

// WithCORS lets the frontend call the API from another origin (e.g. running the frontend dev server
// separately). If the frontend is served by this same Go server (the default — see webassets.go),
// CORS isn't strictly needed, but it doesn't hurt and is left in for flexibility. NOTE: auth is
// cookie-based. If the frontend is ever served from a different origin, "*" for Allow-Origin won't
// work — browsers refuse to send credentialed requests to a wildcard origin. In that case, replace
// "*" with the frontend's exact origin and add Access-Control-Allow-Credentials: true.
func WithCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// WithLogging writes a short line per request.
func WithLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r)

		slog.Debug(fmt.Sprintf("%s %s %s", r.Method, r.URL.Path, time.Since(start)))
	})
}

type ctxKey string

const userContextKey ctxKey = "currentUser"

// userFromContext returns the user requireAuth attached to the request context, or nil if none is
// present.
func userFromContext(r *http.Request) *entity.PublicUser {
	u, _ := r.Context().Value(userContextKey).(*entity.PublicUser)

	return u
}

// sessionToken reads the session cookie, returning "" if it's absent — callers treat a blank token
// the same as any other invalid one.
func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}

	return cookie.Value
}

// requireAuth is the middleware every route but POST /api/auth/login is wrapped with: without a
// valid session, it never reaches next.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		out, err := s.Auth.Authenticate.Execute(
			r.Context(),
			authenticate.Input{
				Token: sessionToken(r),
			},
		)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Authentication required")
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, &out.User)

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}
