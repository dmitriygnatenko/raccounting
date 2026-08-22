package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
)

func TestHandleExportData(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/data/export", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("streams the backup as a downloadable JSON file", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.users.getSettingsFn = func(context.Context, uint64) (entity.UserSettings, error) {
			return entity.UserSettings{}, nil
		}
		deps.currencies.listFn = func(context.Context) ([]entity.Currency, error) { return nil, nil }
		deps.accounts.listFn = func(context.Context) ([]entity.Account, error) { return nil, nil }
		deps.categories.listFn = func(context.Context) ([]entity.Category, error) { return nil, nil }
		deps.tags.listFn = func(context.Context) ([]entity.Tag, error) { return nil, nil }
		deps.transactions.listFn = func(context.Context) ([]entity.Transaction, error) { return nil, nil }
		deps.budgets.listFn = func(context.Context) ([]entity.Budget, error) { return nil, nil }

		req := httptest.NewRequest(http.MethodGet, "/api/data/export", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, "application/json; charset=utf-8", rec.Header().Get("Content-Type"))
		require.Contains(t, rec.Header().Get("Content-Disposition"), "attachment; filename=\"raccounting-backup-")
		require.Contains(t, rec.Body.String(), `"version"`)
	})
}

func TestHandleImportData(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodPost, "/api/data/import", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPost, "/api/data/import", strings.NewReader(`{not json`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("unsupported backup version fails validation with 400", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		body := `{"version":999}`
		req := httptest.NewRequest(http.MethodPost, "/api/data/import", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("empty backup at the current version restores cleanly", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.listFn = func(context.Context) ([]entity.Transaction, error) { return nil, nil }
		deps.budgets.listFn = func(context.Context) ([]entity.Budget, error) { return nil, nil }
		deps.tags.listFn = func(context.Context) ([]entity.Tag, error) { return nil, nil }
		deps.categories.listFn = func(context.Context) ([]entity.Category, error) { return nil, nil }
		deps.accounts.listFn = func(context.Context) ([]entity.Account, error) { return nil, nil }
		deps.currencies.listFn = func(context.Context) ([]entity.Currency, error) { return nil, nil }

		body := `{"version":1,"currencies":[],"accounts":[],"categories":[],"tags":[],"transactions":[],"budgets":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/data/import", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("wipe failure is mapped to 500", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.listFn = func(context.Context) ([]entity.Transaction, error) {
			return nil, errStub
		}

		body := `{"version":1,"currencies":[],"accounts":[],"categories":[],"tags":[],"transactions":[],"budgets":[]}`
		req := httptest.NewRequest(http.MethodPost, "/api/data/import", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}
