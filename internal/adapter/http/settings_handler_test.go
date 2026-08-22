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

func TestHandleGetSettings(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns the signed-in user's settings", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.users.getSettingsFn = func(_ context.Context, id uint64) (entity.UserSettings, error) {
			require.Equal(t, uint64(1), id)
			return entity.UserSettings{Language: "ru"}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/settings", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"language":"ru"}`, rec.Body.String())
	})
}

func TestHandleUpdateSettings(t *testing.T) {
	t.Parallel()

	t.Run("blank language fails validation with 400", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"language":""}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("valid request updates and returns 200", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.users.updateSettingsFn = func(_ context.Context, id uint64, language string) error {
			require.Equal(t, uint64(1), id)
			require.Equal(t, "fr", language)
			return nil
		}

		req := httptest.NewRequest(http.MethodPatch, "/api/settings", strings.NewReader(`{"language":"fr"}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"language":"fr"}`, rec.Body.String())
	})
}
