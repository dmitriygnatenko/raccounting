package app

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	raccountingWeb "raccounting/web"

	httpAPI "raccounting/internal/adapter/http"

	"raccounting/internal/domain/usecase/auth/authenticate"
	"raccounting/internal/domain/usecase/auth/login"
	"raccounting/internal/domain/usecase/auth/logout"
	"raccounting/internal/domain/usecase/auth/updatecredentials"

	accountCreate "raccounting/internal/domain/usecase/account/create"
	accountDelete "raccounting/internal/domain/usecase/account/delete"
	accountList "raccounting/internal/domain/usecase/account/list"
	accountUpdate "raccounting/internal/domain/usecase/account/update"

	categoryCreate "raccounting/internal/domain/usecase/category/create"
	categoryDelete "raccounting/internal/domain/usecase/category/delete"
	categoryList "raccounting/internal/domain/usecase/category/list"
	categoryUpdate "raccounting/internal/domain/usecase/category/update"

	currencyCreate "raccounting/internal/domain/usecase/currency/create"
	currencyDelete "raccounting/internal/domain/usecase/currency/delete"
	currencyList "raccounting/internal/domain/usecase/currency/list"
	currencyUpdate "raccounting/internal/domain/usecase/currency/update"

	transactionCreate "raccounting/internal/domain/usecase/transaction/create"
	transactionDelete "raccounting/internal/domain/usecase/transaction/delete"
	transactionList "raccounting/internal/domain/usecase/transaction/list"
	transactionUpdate "raccounting/internal/domain/usecase/transaction/update"

	transferCreate "raccounting/internal/domain/usecase/transfer/create"
	transferDelete "raccounting/internal/domain/usecase/transfer/delete"

	categoryBudgetList "raccounting/internal/domain/usecase/categorybudget/list"
	categoryBudgetSet "raccounting/internal/domain/usecase/categorybudget/set"

	settingsGet "raccounting/internal/domain/usecase/settings/get"
	settingsUpdate "raccounting/internal/domain/usecase/settings/update"
)

// newServer wires every use case the HTTP API depends on, grouped the same way httpAPI.Server groups
// its routes (auth, accounts, categories, currencies, transactions, transfers, category budgets,
// settings). Keeping this assembly in one function makes it the single place that shows, for any
// given use case, exactly which repositories and services feed it.
func newServer(repos repositories, svcs services, cookieSecure bool) *httpAPI.Server {
	return &httpAPI.Server{
		Auth: httpAPI.AuthUseCases{
			Login:             login.New(repos.Users, repos.Sessions, svcs.Hasher, svcs.Tokens),
			Logout:            logout.New(repos.Sessions),
			Authenticate:      authenticate.New(repos.Sessions, repos.Users),
			UpdateCredentials: updatecredentials.New(repos.Users, svcs.Hasher),
		},
		Accounts: httpAPI.AccountUseCases{
			Create: accountCreate.New(repos.Accounts, repos.Currencies),
			Update: accountUpdate.New(repos.Accounts, repos.Currencies),
			Delete: accountDelete.New(repos.Accounts),
			List:   accountList.New(repos.Accounts),
		},
		Categories: httpAPI.CategoryUseCases{
			Create: categoryCreate.New(repos.Categories),
			Update: categoryUpdate.New(repos.Categories),
			Delete: categoryDelete.New(repos.Categories),
			List:   categoryList.New(repos.Categories),
		},
		Currencies: httpAPI.CurrencyUseCases{
			Create: currencyCreate.New(repos.Currencies),
			Update: currencyUpdate.New(repos.Currencies),
			Delete: currencyDelete.New(repos.Currencies),
			List:   currencyList.New(repos.Currencies),
		},
		Transactions: httpAPI.TransactionUseCases{
			Create: transactionCreate.New(repos.Transactions, repos.Accounts, repos.Categories),
			Update: transactionUpdate.New(repos.Transactions, repos.Accounts, repos.Categories),
			Delete: transactionDelete.New(repos.Transactions),
			List:   transactionList.New(repos.Transactions),
		},
		Transfers: httpAPI.TransferUseCases{
			Create: transferCreate.New(repos.Transfers, repos.Accounts),
			Delete: transferDelete.New(repos.Transfers),
		},
		CategoryBudgets: httpAPI.CategoryBudgetUseCases{
			Set:  categoryBudgetSet.New(repos.CategoryBudgets, repos.Categories),
			List: categoryBudgetList.New(repos.CategoryBudgets),
		},
		Settings: httpAPI.SettingsUseCases{
			Get:    settingsGet.New(repos.Users),
			Update: settingsUpdate.New(repos.Users),
		},
		CookieSecure: cookieSecure,
	}
}

// newHandler registers the API routes and the embedded frontend on one mux, then wraps it with the
// CORS and logging middleware that every request — API or static file — should go through.
func newHandler(srv *httpAPI.Server) (http.Handler, error) {
	mux := http.NewServeMux()
	srv.RegisterRoutes(mux)

	mux.Handle("/", http.FileServer(http.FS(raccountingWeb.WebFiles)))

	return httpAPI.WithLogging(httpAPI.WithCORS(mux)), nil
}

// newHTTPServer builds the server with timeouts tuned for this app rather than net/http's defaults.
func newHTTPServer(addr string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:    addr,
		Handler: handler,
		// ReadHeaderTimeout guards against slow-header attacks (slowloris); ReadTimeout/WriteTimeout
		// stay generous enough to cover a slow connection without cutting it off.
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// shutdownTimeout bounds how long graceful shutdown waits for in-flight requests to finish before
// giving up and forcibly closing whatever connections are still open.
const shutdownTimeout = 10 * time.Second

// runServer serves on httpServer until ctx is canceled, then drains in-flight requests via Shutdown
// instead of cutting them off. A fresh, un-canceled context is used for Shutdown itself since ctx
// just fired as the reason to stop.
func runServer(ctx context.Context, httpServer *http.Server) error {
	serveErr := make(chan error, 1)

	go func() {
		serveErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		return fmt.Errorf("server stopped unexpectedly: %w", err)

	case <-ctx.Done():
		slog.Info("shutting down")

		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()

		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			return fmt.Errorf("failed to shut down cleanly: %w", err)
		}

		return nil
	}
}
