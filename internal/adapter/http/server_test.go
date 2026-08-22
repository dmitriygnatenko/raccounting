package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHandleHealth(t *testing.T) {
	t.Parallel()

	mux, _ := newTestServer()

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

// TestRegisterRoutes_RequireAuth is a smoke test confirming every route but POST /api/auth/login
// is wrapped with requireAuth, exactly as RegisterRoutes documents.
func TestRegisterRoutes_RequireAuth(t *testing.T) {
	t.Parallel()

	protected := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/auth/logout"},
		{http.MethodGet, "/api/auth/me"},
		{http.MethodPatch, "/api/auth/credentials"},
		{http.MethodGet, "/api/accounts"},
		{http.MethodPost, "/api/accounts"},
		{http.MethodGet, "/api/categories"},
		{http.MethodGet, "/api/currencies"},
		{http.MethodGet, "/api/transactions"},
		{http.MethodGet, "/api/transactions/usage"},
		{http.MethodPost, "/api/transfers"},
		{http.MethodGet, "/api/settings"},
		{http.MethodGet, "/api/category-budgets"},
		{http.MethodGet, "/api/tags"},
		{http.MethodGet, "/api/data/export"},
	}

	for _, route := range protected {
		t.Run(route.method+" "+route.path, func(t *testing.T) {
			t.Parallel()

			mux, _ := newTestServer()

			req := httptest.NewRequest(route.method, route.path, nil)
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			require.Equal(t, http.StatusUnauthorized, rec.Code)
		})
	}
}

// TestRegisterRoutes_LoginIsPublic confirms POST /api/auth/login is the one route reachable
// without a session — it still 400s here since no body is sent, but that response only happens if
// requireAuth was skipped.
func TestRegisterRoutes_LoginIsPublic(t *testing.T) {
	t.Parallel()

	mux, _ := newTestServer()

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
}
