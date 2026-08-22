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

func TestHandleListCategories(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns every category as JSON", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.listFn = func(context.Context) ([]entity.Category, error) {
			return []entity.Category{{ID: 1, Name: "Groceries", Type: entity.CategoryTypeExpense, Status: entity.CategoryStatusActive}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/categories", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"Groceries"`)
	})
}

func TestHandleCreateCategory(t *testing.T) {
	t.Parallel()

	t.Run("malformed body is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(`{not json`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("valid request creates the category and returns 201", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.createFn = func(_ context.Context, req port.CategoryCreateRequest) (entity.Category, error) {
			return entity.Category{ID: 3, Name: req.Name, Color: req.Color, Type: req.Type, Status: entity.CategoryStatusActive}, nil
		}

		body := `{"name":"Groceries","type":2,"color":"#ffaa00"}`
		req := httptest.NewRequest(http.MethodPost, "/api/categories", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		require.Contains(t, rec.Body.String(), `"Groceries"`)
	})
}

func TestHandleUpdateCategory(t *testing.T) {
	t.Parallel()

	t.Run("invalid id is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPut, "/api/categories/0", strings.NewReader(`{}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found is mapped to 404", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.updateFn = func(context.Context, port.CategoryUpdateRequest) (entity.Category, error) {
			return entity.Category{}, &domainerror.NotFoundError{Message: "Category not found"}
		}

		body := `{"name":"Groceries","color":"#ffaa00"}`
		req := httptest.NewRequest(http.MethodPut, "/api/categories/9", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleDeleteCategory(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.deleteFn = func(_ context.Context, id uint64) error {
			require.Equal(t, uint64(9), id)
			return nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/categories/9", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("conflict from the use case is mapped to 409", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.deleteFn = func(context.Context, uint64) error {
			return &domainerror.ConflictError{Message: "Category is still in use"}
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/categories/9", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})
}
