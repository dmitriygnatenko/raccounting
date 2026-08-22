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

func TestHandleListCategoryBudgets(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/category-budgets", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("reshapes the flat list into {categoryId: {monthKey: amount}}", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.budgets.listFn = func(context.Context) ([]entity.Budget, error) {
			return []entity.Budget{
				{CategoryID: 1, MonthKey: "2026-01", Amount: 1000},
				{CategoryID: 1, MonthKey: "2026-02", Amount: 2000},
				{CategoryID: 2, MonthKey: "2026-01", Amount: 500},
			}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/category-budgets", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t,
			`{"1":{"2026-01":1000,"2026-02":2000},"2":{"2026-01":500}}`,
			rec.Body.String(),
		)
	})

	t.Run("empty list reshapes into an empty object", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.budgets.listFn = func(context.Context) ([]entity.Budget, error) {
			return nil, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/category-budgets", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{}`, rec.Body.String())
	})
}

func TestHandleSetCategoryBudget(t *testing.T) {
	t.Parallel()

	t.Run("invalid category id is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPut, "/api/category-budgets/abc/2026-01", strings.NewReader(`{"amount":1000}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.JSONEq(t, `{"error":"Invalid category id"}`, rec.Body.String())
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPut, "/api/category-budgets/1/2026-01", strings.NewReader(`{not json`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("valid request sets the budget and returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.existsFn = func(context.Context, uint64) (bool, error) { return true, nil }
		deps.budgets.setFn = func(_ context.Context, req port.BudgetSetRequest) error {
			require.Equal(t, uint64(1), req.CategoryID)
			require.Equal(t, "2026-01", req.MonthKey)
			require.Equal(t, int64(1000), req.Amount)
			return nil
		}

		req := httptest.NewRequest(http.MethodPut, "/api/category-budgets/1/2026-01", strings.NewReader(`{"amount":1000}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("unknown category is mapped to 400", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.categories.existsFn = func(context.Context, uint64) (bool, error) { return false, nil }

		req := httptest.NewRequest(http.MethodPut, "/api/category-budgets/1/2026-01", strings.NewReader(`{"amount":1000}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
