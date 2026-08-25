package http

// This file bundles hand-rolled test doubles for every port.* repository, plus a helper that wires
// them into a fully real Server (real use cases, fake repositories) — the same "fake at the
// repository boundary" style the use case tests use, just one layer up. Handler tests exercise the
// whole stack from an HTTP request down to a fake repository call, through the real mux (so routing,
// requireAuth and path-value parsing are covered too), and back up through the real JSON encoding.

import (
	"context"
	"errors"
	"net/http"
	"time"

	"raccounting/internal/domain/entity"
	"raccounting/internal/port"

	accountCreate "raccounting/internal/domain/usecase/account/create"
	accountDelete "raccounting/internal/domain/usecase/account/delete"
	accountList "raccounting/internal/domain/usecase/account/list"
	accountUpdate "raccounting/internal/domain/usecase/account/update"

	"raccounting/internal/domain/usecase/auth/authenticate"
	"raccounting/internal/domain/usecase/auth/login"
	"raccounting/internal/domain/usecase/auth/logout"
	"raccounting/internal/domain/usecase/auth/updatecredentials"

	categoryCreate "raccounting/internal/domain/usecase/category/create"
	categoryDelete "raccounting/internal/domain/usecase/category/delete"
	categoryList "raccounting/internal/domain/usecase/category/list"
	categoryUpdate "raccounting/internal/domain/usecase/category/update"

	categoryBudgetList "raccounting/internal/domain/usecase/categorybudget/list"
	categoryBudgetSet "raccounting/internal/domain/usecase/categorybudget/set"

	currencyCreate "raccounting/internal/domain/usecase/currency/create"
	currencyDelete "raccounting/internal/domain/usecase/currency/delete"
	currencyList "raccounting/internal/domain/usecase/currency/list"
	currencyUpdate "raccounting/internal/domain/usecase/currency/update"

	dataExport "raccounting/internal/domain/usecase/data/export"
	dataRestore "raccounting/internal/domain/usecase/data/restore"

	settingsGet "raccounting/internal/domain/usecase/settings/get"
	settingsUpdate "raccounting/internal/domain/usecase/settings/update"

	tagCreate "raccounting/internal/domain/usecase/tag/create"
	tagDelete "raccounting/internal/domain/usecase/tag/delete"
	tagList "raccounting/internal/domain/usecase/tag/list"
	tagUpdate "raccounting/internal/domain/usecase/tag/update"

	transactionCreate "raccounting/internal/domain/usecase/transaction/create"
	transactionDelete "raccounting/internal/domain/usecase/transaction/delete"
	transactionList "raccounting/internal/domain/usecase/transaction/list"
	transactionUpdate "raccounting/internal/domain/usecase/transaction/update"
	transactionUsage "raccounting/internal/domain/usecase/transaction/usage"

	transferCreate "raccounting/internal/domain/usecase/transfer/create"
	transferDelete "raccounting/internal/domain/usecase/transfer/delete"
)

// testToken is the session token every authenticated test request carries.
const testToken = "test-session-token"

// errStub is a generic repository failure used across handler tests to exercise the "unknown error
// maps to 500" branch of writeUseCaseError.
var errStub = errors.New("stub failure")

// fakeAccountRepository is a hand-rolled test double for port.AccountRepository.
type fakeAccountRepository struct {
	listFn     func(ctx context.Context) ([]entity.Account, error)
	findByIDFn func(ctx context.Context, id uint64) (entity.Account, error)
	createFn   func(ctx context.Context, req port.AccountCreateRequest) (entity.Account, error)
	updateFn   func(ctx context.Context, req port.AccountUpdateRequest) (entity.Account, error)
	deleteFn   func(ctx context.Context, id uint64) error
}

func (f *fakeAccountRepository) List(ctx context.Context) ([]entity.Account, error) {
	return f.listFn(ctx)
}

func (f *fakeAccountRepository) FindByID(ctx context.Context, id uint64) (entity.Account, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeAccountRepository) Create(ctx context.Context, req port.AccountCreateRequest) (entity.Account, error) {
	return f.createFn(ctx, req)
}

func (f *fakeAccountRepository) Update(ctx context.Context, req port.AccountUpdateRequest) (entity.Account, error) {
	return f.updateFn(ctx, req)
}

func (f *fakeAccountRepository) Delete(ctx context.Context, id uint64) error {
	return f.deleteFn(ctx, id)
}

// fakeCategoryRepository is a hand-rolled test double for port.CategoryRepository.
type fakeCategoryRepository struct {
	listFn   func(ctx context.Context) ([]entity.Category, error)
	existsFn func(ctx context.Context, id uint64) (bool, error)
	createFn func(ctx context.Context, req port.CategoryCreateRequest) (entity.Category, error)
	updateFn func(ctx context.Context, req port.CategoryUpdateRequest) (entity.Category, error)
	deleteFn func(ctx context.Context, id uint64) error
}

func (f *fakeCategoryRepository) List(ctx context.Context) ([]entity.Category, error) {
	return f.listFn(ctx)
}

func (f *fakeCategoryRepository) Exists(ctx context.Context, id uint64) (bool, error) {
	return f.existsFn(ctx, id)
}

func (f *fakeCategoryRepository) Create(ctx context.Context, req port.CategoryCreateRequest) (entity.Category, error) {
	return f.createFn(ctx, req)
}

func (f *fakeCategoryRepository) Update(ctx context.Context, req port.CategoryUpdateRequest) (entity.Category, error) {
	return f.updateFn(ctx, req)
}

func (f *fakeCategoryRepository) Delete(ctx context.Context, id uint64) error {
	return f.deleteFn(ctx, id)
}

// fakeCurrencyRepository is a hand-rolled test double for port.CurrencyRepository.
type fakeCurrencyRepository struct {
	listFn   func(ctx context.Context) ([]entity.Currency, error)
	existsFn func(ctx context.Context, code string) (bool, error)
	createFn func(ctx context.Context, req port.CurrencyCreateRequest) (entity.Currency, error)
	updateFn func(ctx context.Context, req port.CurrencyUpdateRequest) (entity.Currency, error)
	deleteFn func(ctx context.Context, code string) error
}

func (f *fakeCurrencyRepository) List(ctx context.Context) ([]entity.Currency, error) {
	return f.listFn(ctx)
}

func (f *fakeCurrencyRepository) Exists(ctx context.Context, code string) (bool, error) {
	return f.existsFn(ctx, code)
}

func (f *fakeCurrencyRepository) Create(ctx context.Context, req port.CurrencyCreateRequest) (entity.Currency, error) {
	return f.createFn(ctx, req)
}

func (f *fakeCurrencyRepository) Update(ctx context.Context, req port.CurrencyUpdateRequest) (entity.Currency, error) {
	return f.updateFn(ctx, req)
}

func (f *fakeCurrencyRepository) Delete(ctx context.Context, code string) error {
	return f.deleteFn(ctx, code)
}

// fakeBudgetRepository is a hand-rolled test double for port.BudgetRepository.
type fakeBudgetRepository struct {
	listFn func(ctx context.Context) ([]entity.Budget, error)
	setFn  func(ctx context.Context, req port.BudgetSetRequest) error
}

func (f *fakeBudgetRepository) List(ctx context.Context) ([]entity.Budget, error) {
	return f.listFn(ctx)
}

func (f *fakeBudgetRepository) Set(ctx context.Context, req port.BudgetSetRequest) error {
	return f.setFn(ctx, req)
}

// fakeTagRepository is a hand-rolled test double for port.TagRepository.
type fakeTagRepository struct {
	listFn      func(ctx context.Context) ([]entity.Tag, error)
	findByIDsFn func(ctx context.Context, ids []uint64) ([]entity.Tag, error)
	createFn    func(ctx context.Context, req port.TagCreateRequest) (entity.Tag, error)
	updateFn    func(ctx context.Context, req port.TagUpdateRequest) (entity.Tag, error)
	deleteFn    func(ctx context.Context, id uint64) error
}

func (f *fakeTagRepository) List(ctx context.Context) ([]entity.Tag, error) {
	return f.listFn(ctx)
}

func (f *fakeTagRepository) FindByIDs(ctx context.Context, ids []uint64) ([]entity.Tag, error) {
	return f.findByIDsFn(ctx, ids)
}

func (f *fakeTagRepository) Create(ctx context.Context, req port.TagCreateRequest) (entity.Tag, error) {
	return f.createFn(ctx, req)
}

func (f *fakeTagRepository) Update(ctx context.Context, req port.TagUpdateRequest) (entity.Tag, error) {
	return f.updateFn(ctx, req)
}

func (f *fakeTagRepository) Delete(ctx context.Context, id uint64) error {
	return f.deleteFn(ctx, id)
}

// fakeTransactionRepository is a hand-rolled test double for port.TransactionRepository.
type fakeTransactionRepository struct {
	listFn           func(ctx context.Context) ([]entity.Transaction, error)
	listFilteredFn   func(ctx context.Context, filter port.TransactionListFilter) (port.TransactionListResult, error)
	usageFn          func(ctx context.Context) (port.TransactionUsage, error)
	findByIDFn       func(ctx context.Context, id uint64) (entity.Transaction, error)
	createFn         func(ctx context.Context, req port.TransactionCreateRequest) (entity.Transaction, error)
	updateFn         func(ctx context.Context, req port.TransactionUpdateRequest) (entity.Transaction, error)
	deleteFn         func(ctx context.Context, id uint64) error
	createTransferFn func(ctx context.Context, req port.TransferCreateRequest) (port.TransferResult, error)
	deleteTransferFn func(ctx context.Context, id uint64) (bool, error)
}

func (f *fakeTransactionRepository) List(ctx context.Context) ([]entity.Transaction, error) {
	return f.listFn(ctx)
}

func (f *fakeTransactionRepository) ListFiltered(ctx context.Context, filter port.TransactionListFilter) (port.TransactionListResult, error) {
	return f.listFilteredFn(ctx, filter)
}

func (f *fakeTransactionRepository) Usage(ctx context.Context) (port.TransactionUsage, error) {
	return f.usageFn(ctx)
}

func (f *fakeTransactionRepository) FindByID(ctx context.Context, id uint64) (entity.Transaction, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeTransactionRepository) Create(ctx context.Context, req port.TransactionCreateRequest) (entity.Transaction, error) {
	return f.createFn(ctx, req)
}

func (f *fakeTransactionRepository) Update(ctx context.Context, req port.TransactionUpdateRequest) (entity.Transaction, error) {
	return f.updateFn(ctx, req)
}

func (f *fakeTransactionRepository) Delete(ctx context.Context, id uint64) error {
	return f.deleteFn(ctx, id)
}

func (f *fakeTransactionRepository) CreateTransfer(ctx context.Context, req port.TransferCreateRequest) (port.TransferResult, error) {
	return f.createTransferFn(ctx, req)
}

func (f *fakeTransactionRepository) DeleteTransfer(ctx context.Context, id uint64) (bool, error) {
	return f.deleteTransferFn(ctx, id)
}

// fakeUserRepository is a hand-rolled test double for port.UserRepository.
type fakeUserRepository struct {
	findByUsernameFn     func(ctx context.Context, username string) (entity.User, error)
	findByIDFn           func(ctx context.Context, id uint64) (entity.User, error)
	createFn             func(ctx context.Context, req port.UserCreateRequest) (uint64, error)
	updateUsernameFn     func(ctx context.Context, id uint64, username string) error
	updatePasswordHashFn func(ctx context.Context, id uint64, hash string) error
	getSettingsFn        func(ctx context.Context, id uint64) (entity.UserSettings, error)
	updateSettingsFn     func(ctx context.Context, id uint64, settings entity.UserSettings) error
	countFn              func(ctx context.Context) (int, error)
}

func (f *fakeUserRepository) FindByUsername(ctx context.Context, username string) (entity.User, error) {
	return f.findByUsernameFn(ctx, username)
}

func (f *fakeUserRepository) FindByID(ctx context.Context, id uint64) (entity.User, error) {
	return f.findByIDFn(ctx, id)
}

func (f *fakeUserRepository) Create(ctx context.Context, req port.UserCreateRequest) (uint64, error) {
	return f.createFn(ctx, req)
}

func (f *fakeUserRepository) UpdateUsername(ctx context.Context, id uint64, username string) error {
	return f.updateUsernameFn(ctx, id, username)
}

func (f *fakeUserRepository) UpdatePasswordHash(ctx context.Context, id uint64, hash string) error {
	return f.updatePasswordHashFn(ctx, id, hash)
}

func (f *fakeUserRepository) GetSettings(ctx context.Context, id uint64) (entity.UserSettings, error) {
	return f.getSettingsFn(ctx, id)
}

func (f *fakeUserRepository) UpdateSettings(ctx context.Context, id uint64, settings entity.UserSettings) error {
	return f.updateSettingsFn(ctx, id, settings)
}

func (f *fakeUserRepository) Count(ctx context.Context) (int, error) {
	return f.countFn(ctx)
}

// fakeSessionRepository is a hand-rolled test double for port.SessionRepository.
type fakeSessionRepository struct {
	createFn        func(ctx context.Context, session entity.Session) error
	findByTokenFn   func(ctx context.Context, token string) (entity.Session, error)
	deleteFn        func(ctx context.Context, token string) error
	deleteExpiredFn func(ctx context.Context, now time.Time) (int64, error)
}

func (f *fakeSessionRepository) Create(ctx context.Context, session entity.Session) error {
	return f.createFn(ctx, session)
}

func (f *fakeSessionRepository) FindByToken(ctx context.Context, token string) (entity.Session, error) {
	return f.findByTokenFn(ctx, token)
}

func (f *fakeSessionRepository) Delete(ctx context.Context, token string) error {
	return f.deleteFn(ctx, token)
}

func (f *fakeSessionRepository) DeleteExpired(ctx context.Context, now time.Time) (int64, error) {
	return f.deleteExpiredFn(ctx, now)
}

// fakePasswordHasher is a hand-rolled test double for port.PasswordHasher.
type fakePasswordHasher struct {
	hashFn    func(password string) (string, error)
	compareFn func(hash, password string) bool
}

func (f *fakePasswordHasher) Hash(password string) (string, error) {
	return f.hashFn(password)
}

func (f *fakePasswordHasher) Compare(hash, password string) bool {
	return f.compareFn(hash, password)
}

// fakeTokenGenerator is a hand-rolled test double for port.TokenGenerator.
type fakeTokenGenerator struct {
	newTokenFn func() (string, error)
}

func (f *fakeTokenGenerator) NewToken() (string, error) {
	return f.newTokenFn()
}

// testDeps bundles every fake repository a fully wired Server depends on, so a test can stub just
// the ones its scenario touches.
type testDeps struct {
	accounts     *fakeAccountRepository
	categories   *fakeCategoryRepository
	currencies   *fakeCurrencyRepository
	budgets      *fakeBudgetRepository
	tags         *fakeTagRepository
	transactions *fakeTransactionRepository
	users        *fakeUserRepository
	sessions     *fakeSessionRepository
	hasher       *fakePasswordHasher
	tokens       *fakeTokenGenerator
}

// newTestServer builds a Server wired entirely from real use cases backed by fresh, unstubbed fakes,
// and registers its routes onto a *http.ServeMux — so tests exercise routing, requireAuth and
// path-value parsing exactly as production does. Each fake method panics with "not stubbed" until a
// test sets the matching *Fn field.
func newTestServer() (*http.ServeMux, *testDeps) {
	deps := &testDeps{
		accounts:     &fakeAccountRepository{},
		categories:   &fakeCategoryRepository{},
		currencies:   &fakeCurrencyRepository{},
		budgets:      &fakeBudgetRepository{},
		tags:         &fakeTagRepository{},
		transactions: &fakeTransactionRepository{},
		users:        &fakeUserRepository{},
		sessions:     &fakeSessionRepository{},
		hasher:       &fakePasswordHasher{},
		tokens:       &fakeTokenGenerator{},
	}

	s := &Server{
		Auth: AuthUseCases{
			Login:             login.New(deps.users, deps.sessions, deps.hasher, deps.tokens),
			Logout:            logout.New(deps.sessions),
			Authenticate:      authenticate.New(deps.sessions, deps.users),
			UpdateCredentials: updatecredentials.New(deps.users, deps.hasher),
		},
		Accounts: AccountUseCases{
			Create: accountCreate.New(deps.accounts, deps.currencies),
			Update: accountUpdate.New(deps.accounts, deps.currencies),
			Delete: accountDelete.New(deps.accounts),
			List:   accountList.New(deps.accounts),
		},
		Categories: CategoryUseCases{
			Create: categoryCreate.New(deps.categories),
			Update: categoryUpdate.New(deps.categories),
			Delete: categoryDelete.New(deps.categories),
			List:   categoryList.New(deps.categories),
		},
		Currencies: CurrencyUseCases{
			Create: currencyCreate.New(deps.currencies),
			Update: currencyUpdate.New(deps.currencies),
			Delete: currencyDelete.New(deps.currencies),
			List:   currencyList.New(deps.currencies),
		},
		Transactions: TransactionUseCases{
			Create: transactionCreate.New(deps.transactions, deps.accounts, deps.categories, deps.tags),
			Update: transactionUpdate.New(deps.transactions, deps.accounts, deps.categories, deps.tags),
			Delete: transactionDelete.New(deps.transactions),
			List:   transactionList.New(deps.transactions),
			Usage:  transactionUsage.New(deps.transactions),
		},
		Transfers: TransferUseCases{
			Create: transferCreate.New(deps.transactions, deps.accounts),
			Delete: transferDelete.New(deps.transactions),
		},
		Budgets: CategoryBudgetUseCases{
			Set:  categoryBudgetSet.New(deps.budgets, deps.categories),
			List: categoryBudgetList.New(deps.budgets),
		},
		Settings: SettingsUseCases{
			Get:    settingsGet.New(deps.users),
			Update: settingsUpdate.New(deps.users),
		},
		Tags: TagUseCases{
			Create: tagCreate.New(deps.tags),
			Update: tagUpdate.New(deps.tags),
			Delete: tagDelete.New(deps.tags),
			List:   tagList.New(deps.tags),
		},
		Data: DataUseCases{
			Export:  dataExport.New(deps.users, deps.accounts, deps.categories, deps.currencies, deps.tags, deps.transactions, deps.budgets),
			Restore: dataRestore.New(deps.users, deps.accounts, deps.categories, deps.currencies, deps.tags, deps.transactions, deps.budgets),
		},
		CookieSecure: false,
	}

	mux := http.NewServeMux()
	s.RegisterRoutes(mux)

	return mux, deps
}

// stubAuthenticated makes testToken resolve, through requireAuth, to user — every route but
// POST /api/auth/login requires this before a test request carrying authCookie() will reach its
// handler.
func stubAuthenticated(deps *testDeps, user entity.User) {
	deps.sessions.findByTokenFn = func(context.Context, string) (entity.Session, error) {
		return entity.Session{
			Token:     testToken,
			UserID:    user.ID,
			ExpiresAt: time.Now().UTC().Add(time.Hour),
		}, nil
	}
	deps.users.findByIDFn = func(context.Context, uint64) (entity.User, error) {
		return user, nil
	}
}

// authCookie is the session cookie a request needs to authenticate against stubAuthenticated.
func authCookie() *http.Cookie {
	return &http.Cookie{Name: sessionCookieName, Value: testToken}
}
