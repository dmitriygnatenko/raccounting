package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
)

func TestHandleListTags(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns every tag as JSON", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.tags.listFn = func(context.Context) ([]entity.Tag, error) {
			return []entity.Tag{{ID: 1, Name: "vacation", Color: "#ffaa00"}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"vacation"`)
	})
}

func TestHandleCreateTag(t *testing.T) {
	t.Parallel()

	t.Run("blank name fails validation with 400", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPost, "/api/tags", strings.NewReader(`{"name":""}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.JSONEq(t, `{"error":"Please enter a tag name"}`, rec.Body.String())
	})

	t.Run("valid request creates the tag and returns 201", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.tags.createFn = func(_ context.Context, req port.TagCreateRequest) (entity.Tag, error) {
			return entity.Tag{ID: 2, Name: req.Name, Color: req.Color}, nil
		}

		body := `{"name":"vacation","color":"#ffaa00"}`
		req := httptest.NewRequest(http.MethodPost, "/api/tags", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		require.Contains(t, rec.Body.String(), `"vacation"`)
	})
}

func TestHandleUpdateTag(t *testing.T) {
	t.Parallel()

	t.Run("not found is mapped to 404", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.tags.updateFn = func(context.Context, port.TagUpdateRequest) (entity.Tag, error) {
			return entity.Tag{}, &domainerror.NotFoundError{Message: "Tag not found"}
		}

		body := `{"name":"vacation","color":"#ffaa00"}`
		req := httptest.NewRequest(http.MethodPut, "/api/tags/9", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleDeleteTag(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.tags.deleteFn = func(_ context.Context, id uint64) error {
			require.Equal(t, uint64(9), id)
			return nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/tags/9", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})
}
