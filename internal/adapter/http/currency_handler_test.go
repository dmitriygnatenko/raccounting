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

func TestHandleListCurrencies(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/currencies", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("returns every currency as JSON", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.listFn = func(context.Context) ([]entity.Currency, error) {
			return []entity.Currency{{Code: "RUB", Symbol: "₽", Name: "Russian Ruble", Rate: 1, Status: entity.CurrencyStatusActive}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/currencies", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"RUB"`)
	})
}

func TestHandleCreateCurrency(t *testing.T) {
	t.Parallel()

	t.Run("conflicting code is mapped to 409", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.createFn = func(context.Context, port.CurrencyCreateRequest) (entity.Currency, error) {
			return entity.Currency{}, &domainerror.ConflictError{Message: "Currency already exists"}
		}

		body := `{"code":"RUB","symbol":"₽","name":"Russian Ruble","rate":1}`
		req := httptest.NewRequest(http.MethodPost, "/api/currencies", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})

	t.Run("valid request creates the currency and returns 201", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.createFn = func(_ context.Context, req port.CurrencyCreateRequest) (entity.Currency, error) {
			return entity.Currency{Code: req.Code, Symbol: req.Symbol, Name: req.Name, Rate: req.Rate, Status: entity.CurrencyStatusActive}, nil
		}

		body := `{"code":"USD","symbol":"$","name":"US Dollar","rate":90}`
		req := httptest.NewRequest(http.MethodPost, "/api/currencies", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		require.Contains(t, rec.Body.String(), `"USD"`)
	})
}

func TestHandleUpdateCurrency(t *testing.T) {
	t.Parallel()

	t.Run("not found is mapped to 404", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.updateFn = func(context.Context, port.CurrencyUpdateRequest) (entity.Currency, error) {
			return entity.Currency{}, &domainerror.NotFoundError{Message: "Currency not found"}
		}

		body := `{"symbol":"$","name":"US Dollar","rate":90}`
		req := httptest.NewRequest(http.MethodPut, "/api/currencies/USD", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})

	t.Run("valid request updates and returns 200", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.updateFn = func(_ context.Context, req port.CurrencyUpdateRequest) (entity.Currency, error) {
			require.Equal(t, "USD", req.Code)
			return entity.Currency{Code: req.Code, Symbol: req.Symbol, Name: req.Name, Rate: req.Rate, Status: entity.CurrencyStatusActive}, nil
		}

		body := `{"symbol":"$","name":"US Dollar","rate":91.5}`
		req := httptest.NewRequest(http.MethodPut, "/api/currencies/USD", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestHandleDeleteCurrency(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.currencies.deleteFn = func(_ context.Context, code string) error {
			require.Equal(t, "USD", code)
			return nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/currencies/USD", nil)
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
		deps.currencies.deleteFn = func(context.Context, string) error {
			return &domainerror.ConflictError{Message: "Currency is still in use"}
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/currencies/USD", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})
}
