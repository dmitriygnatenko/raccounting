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

func TestHandleListAccounts(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns every account as JSON", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.listFn = func(context.Context) ([]entity.Account, error) {
			return []entity.Account{{ID: 1, Name: "Cash", CurrencyCode: "RUB", Status: entity.AccountStatusActive}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"Cash"`)
	})

	t.Run("use case failure is mapped to 500", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.listFn = func(context.Context) ([]entity.Account, error) {
			return nil, errStub
		}

		req := httptest.NewRequest(http.MethodGet, "/api/accounts", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})
}

func TestHandleCreateAccount(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("malformed body is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(`{not json`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("validation failure from the use case is passed through as 400", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(`{"name":"","type":1,"currency":"RUB"}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.JSONEq(t, `{"error":"Please enter an account name"}`, rec.Body.String())
	})

	t.Run("valid request creates the account and returns 201", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.existsFn = func(context.Context, string) (bool, error) { return true, nil }
		deps.accounts.createFn = func(_ context.Context, req port.AccountCreateRequest) (entity.Account, error) {
			return entity.Account{ID: 5, Name: req.Name, Type: req.Type, CurrencyCode: req.CurrencyCode, Balance: req.Balance, Status: entity.AccountStatusActive}, nil
		}

		body := `{"name":"Cash","type":1,"currency":"RUB","balance":1000}`
		req := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		require.Contains(t, rec.Body.String(), `"Cash"`)
	})
}

func TestHandleUpdateAccount(t *testing.T) {
	t.Parallel()

	t.Run("invalid id is rejected", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		req := httptest.NewRequest(http.MethodPut, "/api/accounts/abc", strings.NewReader(`{}`))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("not found account is mapped to 404", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.existsFn = func(context.Context, string) (bool, error) { return true, nil }
		deps.accounts.updateFn = func(context.Context, port.AccountUpdateRequest) (entity.Account, error) {
			return entity.Account{}, &domainerror.NotFoundError{Message: "Account not found"}
		}

		body := `{"name":"Cash","type":1,"currency":"RUB"}`
		req := httptest.NewRequest(http.MethodPut, "/api/accounts/9", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("valid request updates and returns 200", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.existsFn = func(context.Context, string) (bool, error) { return true, nil }
		deps.accounts.updateFn = func(_ context.Context, req port.AccountUpdateRequest) (entity.Account, error) {
			require.Equal(t, uint64(9), req.ID)
			return entity.Account{ID: req.ID, Name: req.Name, CurrencyCode: req.CurrencyCode, Status: entity.AccountStatusActive}, nil
		}

		body := `{"name":"Savings","type":4,"currency":"RUB"}`
		req := httptest.NewRequest(http.MethodPut, "/api/accounts/9", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"Savings"`)
	})
}

func TestHandleDeleteAccount(t *testing.T) {
	t.Parallel()

	t.Run("conflict from the use case is mapped to 409", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.deleteFn = func(context.Context, uint64) error {
			return &domainerror.ConflictError{Message: "Account is still in use"}
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/accounts/9", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("valid delete returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.deleteFn = func(_ context.Context, id uint64) error {
			require.Equal(t, uint64(9), id)
			return nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/accounts/9", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})
}
