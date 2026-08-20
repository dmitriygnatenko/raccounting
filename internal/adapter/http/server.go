// Package http is the driving adapter: it exposes the application's use cases over HTTP, translating
// requests into use case Input values and use case Output/errors back into JSON responses. It owns
// nothing that looks like business logic — validation, persistence and side effects all live behind
// the use cases it calls.
package http

import (
	"net/http"

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

	dataExport "raccounting/internal/domain/usecase/data/export"
	dataRestore "raccounting/internal/domain/usecase/data/restore"

	settingsGet "raccounting/internal/domain/usecase/settings/get"
	settingsUpdate "raccounting/internal/domain/usecase/settings/update"

	tagCreate "raccounting/internal/domain/usecase/tag/create"
	tagDelete "raccounting/internal/domain/usecase/tag/delete"
	tagList "raccounting/internal/domain/usecase/tag/list"
	tagUpdate "raccounting/internal/domain/usecase/tag/update"
)

// AuthUseCases collects the use cases behind the /api/auth routes.
type AuthUseCases struct {
	Login             *login.UseCase
	Logout            *logout.UseCase
	Authenticate      *authenticate.UseCase
	UpdateCredentials *updatecredentials.UseCase
}

// AccountUseCases collects the use cases behind the /api/accounts routes.
type AccountUseCases struct {
	Create *accountCreate.UseCase
	Update *accountUpdate.UseCase
	Delete *accountDelete.UseCase
	List   *accountList.UseCase
}

// CategoryUseCases collects the use cases behind the /api/categories routes.
type CategoryUseCases struct {
	Create *categoryCreate.UseCase
	Update *categoryUpdate.UseCase
	Delete *categoryDelete.UseCase
	List   *categoryList.UseCase
}

// CurrencyUseCases collects the use cases behind the /api/currencies routes.
type CurrencyUseCases struct {
	Create *currencyCreate.UseCase
	Update *currencyUpdate.UseCase
	Delete *currencyDelete.UseCase
	List   *currencyList.UseCase
}

// TransactionUseCases collects the use cases behind the /api/transactions routes.
type TransactionUseCases struct {
	Create *transactionCreate.UseCase
	Update *transactionUpdate.UseCase
	Delete *transactionDelete.UseCase
	List   *transactionList.UseCase
}

// TransferUseCases collects the use cases behind the /api/transfers routes.
type TransferUseCases struct {
	Create *transferCreate.UseCase
	Delete *transferDelete.UseCase
}

// CategoryBudgetUseCases collects the use cases behind the /api/category-budgets routes.
type CategoryBudgetUseCases struct {
	Set  *categoryBudgetSet.UseCase
	List *categoryBudgetList.UseCase
}

// DataUseCases collects the use cases behind the /api/data routes (full backup export/import).
type DataUseCases struct {
	Export  *dataExport.UseCase
	Restore *dataRestore.UseCase
}

// SettingsUseCases collects the use cases behind the /api/settings routes.
type SettingsUseCases struct {
	Get    *settingsGet.UseCase
	Update *settingsUpdate.UseCase
}

// TagUseCases collects the use cases behind the /api/tags routes.
type TagUseCases struct {
	Create *tagCreate.UseCase
	Update *tagUpdate.UseCase
	Delete *tagDelete.UseCase
	List   *tagList.UseCase
}

// Server holds every use case the API surfaces, plus the handful of settings the HTTP layer itself
// is responsible for (cookie flags).
type Server struct {
	Auth         AuthUseCases
	Accounts     AccountUseCases
	Categories   CategoryUseCases
	Currencies   CurrencyUseCases
	Transactions TransactionUseCases
	Transfers    TransferUseCases
	Budgets      CategoryBudgetUseCases
	Settings     SettingsUseCases
	Tags         TagUseCases
	Data         DataUseCases

	// CookieSecure sets the session cookie's Secure flag — true once the app is served over HTTPS.
	CookieSecure bool
}

// RegisterRoutes wires every endpoint onto mux, matching the plan's API surface exactly (paths,
// methods, auth requirements). Every route is wrapped with requireAuth except POST /api/auth/login.
func (s *Server) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/health", s.handleHealth)

	mux.HandleFunc("POST /api/auth/login", s.handleLogin)
	mux.HandleFunc("POST /api/auth/logout", s.requireAuth(s.handleLogout))
	mux.HandleFunc("GET /api/auth/me", s.requireAuth(s.handleMe))
	mux.HandleFunc("PATCH /api/auth/credentials", s.requireAuth(s.handleUpdateCredentials))

	mux.HandleFunc("GET /api/accounts", s.requireAuth(s.handleListAccounts))
	mux.HandleFunc("POST /api/accounts", s.requireAuth(s.handleCreateAccount))
	mux.HandleFunc("PUT /api/accounts/{id}", s.requireAuth(s.handleUpdateAccount))
	mux.HandleFunc("DELETE /api/accounts/{id}", s.requireAuth(s.handleDeleteAccount))

	mux.HandleFunc("GET /api/categories", s.requireAuth(s.handleListCategories))
	mux.HandleFunc("POST /api/categories", s.requireAuth(s.handleCreateCategory))
	mux.HandleFunc("PUT /api/categories/{id}", s.requireAuth(s.handleUpdateCategory))
	mux.HandleFunc("DELETE /api/categories/{id}", s.requireAuth(s.handleDeleteCategory))

	mux.HandleFunc("GET /api/currencies", s.requireAuth(s.handleListCurrencies))
	mux.HandleFunc("POST /api/currencies", s.requireAuth(s.handleCreateCurrency))
	mux.HandleFunc("PUT /api/currencies/{code}", s.requireAuth(s.handleUpdateCurrency))
	mux.HandleFunc("DELETE /api/currencies/{code}", s.requireAuth(s.handleDeleteCurrency))

	mux.HandleFunc("GET /api/transactions", s.requireAuth(s.handleListTransactions))
	mux.HandleFunc("POST /api/transactions", s.requireAuth(s.handleCreateTransaction))
	mux.HandleFunc("PUT /api/transactions/{id}", s.requireAuth(s.handleUpdateTransaction))
	mux.HandleFunc("DELETE /api/transactions/{id}", s.requireAuth(s.handleDeleteTransaction))

	mux.HandleFunc("POST /api/transfers", s.requireAuth(s.handleCreateTransfer))
	mux.HandleFunc("DELETE /api/transfers/{id}", s.requireAuth(s.handleDeleteTransfer))

	mux.HandleFunc("GET /api/settings", s.requireAuth(s.handleGetSettings))
	mux.HandleFunc("PATCH /api/settings", s.requireAuth(s.handleUpdateSettings))

	mux.HandleFunc("GET /api/category-budgets", s.requireAuth(s.handleListCategoryBudgets))
	mux.HandleFunc("PUT /api/category-budgets/{categoryId}/{monthKey}", s.requireAuth(s.handleSetCategoryBudget))

	mux.HandleFunc("GET /api/tags", s.requireAuth(s.handleListTags))
	mux.HandleFunc("POST /api/tags", s.requireAuth(s.handleCreateTag))
	mux.HandleFunc("PUT /api/tags/{id}", s.requireAuth(s.handleUpdateTag))
	mux.HandleFunc("DELETE /api/tags/{id}", s.requireAuth(s.handleDeleteTag))

	mux.HandleFunc("GET /api/data/export", s.requireAuth(s.handleExportData))
	mux.HandleFunc("POST /api/data/import", s.requireAuth(s.handleImportData))
}

// handleHealth handles GET /api/health — a trivial liveness check.
func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
