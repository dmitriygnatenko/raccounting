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

func TestHandleListTransactions(t *testing.T) {
	t.Parallel()

	t.Run("unauthenticated request is rejected", func(t *testing.T) {
		t.Parallel()

		mux, _ := newTestServer()

		req := httptest.NewRequest(http.MethodGet, "/api/transactions", nil)
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("no filters returns a page as JSON", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.listFilteredFn = func(context.Context, port.TransactionListFilter) (port.TransactionListResult, error) {
			return port.TransactionListResult{
				Transactions:   []entity.Transaction{{ID: 1, AccountID: 1, Type: entity.TransactionTypeExpense, Amount: -500, CurrencyCode: "RUB"}},
				TotalCount:     1,
				SumsByCurrency: map[string]int64{"RUB": -500},
			}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/transactions", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.Contains(t, rec.Body.String(), `"totalCount":1`)
	})

	tests := []struct {
		name  string
		query string
	}{
		{name: "malformed dateFrom", query: "dateFrom=not-a-date"},
		{name: "malformed dateTo", query: "dateTo=not-a-date"},
		{name: "malformed accountId", query: "accountId=abc"},
		{name: "zero accountId", query: "accountId=0"},
		{name: "malformed categoryId", query: "categoryId=abc"},
		{name: "malformed tagId", query: "tagId=abc"},
		{name: "malformed type", query: "type=abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name+" is rejected with 400", func(t *testing.T) {
			t.Parallel()

			mux, deps := newTestServer()
			stubAuthenticated(deps, entity.User{ID: 1})

			req := httptest.NewRequest(http.MethodGet, "/api/transactions?"+tt.query, nil)
			req.AddCookie(authCookie())
			rec := httptest.NewRecorder()

			mux.ServeHTTP(rec, req)

			require.Equal(t, http.StatusBadRequest, rec.Code)
		})
	}

	t.Run("filters are forwarded to the use case", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.listFilteredFn = func(_ context.Context, filter port.TransactionListFilter) (port.TransactionListResult, error) {
			require.NotNil(t, filter.AccountID)
			require.Equal(t, uint64(3), *filter.AccountID)
			require.NotNil(t, filter.DateFrom)
			require.Equal(t, "2026-01-01", filter.DateFrom.Format(entity.DateLayout))
			require.Equal(t, "groceries", filter.Search)
			require.Equal(t, 2, filter.Page)
			require.Equal(t, 10, filter.PageSize)

			return port.TransactionListResult{}, nil
		}

		req := httptest.NewRequest(http.MethodGet,
			"/api/transactions?accountId=3&dateFrom=2026-01-01&search=groceries&page=2&pageSize=10", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
	})
}

func TestHandleTransactionUsage(t *testing.T) {
	t.Parallel()

	t.Run("returns account/category ids referenced by any transaction", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.usageFn = func(context.Context) (port.TransactionUsage, error) {
			return port.TransactionUsage{AccountIDs: []uint64{1, 2}, CategoryIDs: []uint64{5}}, nil
		}

		req := httptest.NewRequest(http.MethodGet, "/api/transactions/usage", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"accountIds":[1,2],"categoryIds":[5]}`, rec.Body.String())
	})
}

func TestHandleCreateTransaction(t *testing.T) {
	t.Parallel()

	t.Run("negative amount is created as an expense", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
			return entity.Account{ID: id, CurrencyCode: "RUB"}, nil
		}
		deps.transactions.createFn = func(_ context.Context, req port.TransactionCreateRequest) (entity.Transaction, error) {
			require.Equal(t, entity.TransactionTypeExpense, req.Type)
			return entity.Transaction{ID: 1, AccountID: req.AccountID, Type: req.Type, Amount: req.Amount, CurrencyCode: "RUB"}, nil
		}

		body := `{"accountId":1,"amount":-500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPost, "/api/transactions", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("positive amount is created as income", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
			return entity.Account{ID: id, CurrencyCode: "RUB"}, nil
		}
		deps.transactions.createFn = func(_ context.Context, req port.TransactionCreateRequest) (entity.Transaction, error) {
			require.Equal(t, entity.TransactionTypeIncome, req.Type)
			return entity.Transaction{ID: 1, AccountID: req.AccountID, Type: req.Type, Amount: req.Amount, CurrencyCode: "RUB"}, nil
		}

		body := `{"accountId":1,"amount":500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPost, "/api/transactions", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusCreated, rec.Code)
	})

	t.Run("account balance conflict is mapped to 409", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.accounts.findByIDFn = func(_ context.Context, id uint64) (entity.Account, error) {
			return entity.Account{ID: id, CurrencyCode: "RUB"}, nil
		}
		deps.transactions.createFn = func(context.Context, port.TransactionCreateRequest) (entity.Transaction, error) {
			return entity.Transaction{}, &domainerror.ConflictError{Message: "Insufficient balance"}
		}

		body := `{"accountId":1,"amount":-500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPost, "/api/transactions", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusConflict, rec.Code)
	})
}

func TestHandleUpdateTransaction(t *testing.T) {
	t.Parallel()

	t.Run("not found is mapped to 404", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.findByIDFn = func(context.Context, uint64) (entity.Transaction, error) {
			return entity.Transaction{}, &domainerror.NotFoundError{Message: "Transaction not found"}
		}

		body := `{"accountId":1,"amount":-500,"date":"2026-01-01"}`
		req := httptest.NewRequest(http.MethodPut, "/api/transactions/9", strings.NewReader(body))
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusNotFound, rec.Code)
	})
}

func TestHandleDeleteTransaction(t *testing.T) {
	t.Parallel()

	t.Run("valid delete returns ok", func(t *testing.T) {
		t.Parallel()

		mux, deps := newTestServer()
		stubAuthenticated(deps, entity.User{ID: 1})
		deps.transactions.findByIDFn = func(_ context.Context, id uint64) (entity.Transaction, error) {
			return entity.Transaction{ID: id, AccountID: 1, Type: entity.TransactionTypeExpense, Amount: -500}, nil
		}
		deps.transactions.deleteFn = func(_ context.Context, id uint64) error {
			require.Equal(t, uint64(9), id)
			return nil
		}

		req := httptest.NewRequest(http.MethodDelete, "/api/transactions/9", nil)
		req.AddCookie(authCookie())
		rec := httptest.NewRecorder()

		mux.ServeHTTP(rec, req)

		require.Equal(t, http.StatusOK, rec.Code)
		require.JSONEq(t, `{"ok":true}`, rec.Body.String())
	})
}
