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

func TestHandleCreateTransfer(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(`{}`))
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("same source and destination account fails validation with 400", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})

		body := `{"fromAccountId":1,"toAccountId":1,"amount":500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusBadRequest, rec.Code)
		require.JSONEq(t, `{"error":"Choose two different accounts"}`, rec.Body.String())
	})

	t.Run("valid transfer returns both legs with 201", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
			return entity.Account{ID: id, CurrencyCode: "RUB"}, nil
		}
		deps.transactions.createTransferFn = func(_ context.Context, req port.TransferCreateRequest) (port.TransferResult, error) {
			require.Equal(t, uint64(1), req.FromAccountID)
			require.Equal(t, uint64(2), req.ToAccountID)
			return port.TransferResult{
				LegFrom: entity.Transaction{ID: 10, AccountID: 1, Type: entity.TransactionTypeTransfer, Amount: -500},
				LegTo:   entity.Transaction{ID: 11, AccountID: 2, Type: entity.TransactionTypeTransfer, Amount: 500},
			}, nil
		}

		body := `{"fromAccountId":1,"toAccountId":2,"amount":500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
		require.Contains(t, rec.Body.String(), `"legFrom"`)
		require.Contains(t, rec.Body.String(), `"legTo"`)
	})

	t.Run("balance conflict is mapped to 409", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
			return entity.Account{ID: id, CurrencyCode: "RUB"}, nil
		}
		deps.transactions.createTransferFn = func(context.Context, port.TransferCreateRequest) (port.TransferResult, error) {
			return port.TransferResult{}, &domainerror.ConflictError{Message: "This would overdraw the account"}
		}

		body := `{"fromAccountId":1,"toAccountId":2,"amount":500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPost, "/api/transfers", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestHandleDeleteTransfer(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.deleteTransferFn = func(_ context.Context, id uint64) (bool, error) {
			require.Equal(t, uint64(10), id)
			return true, nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/transfers/10", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})

	t.Run("not found is mapped to 404", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.deleteTransferFn = func(context.Context, uint64) (bool, error) {
			return false, nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/transfers/10", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}
