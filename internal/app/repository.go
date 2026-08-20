package app

import (
	accountRepo "raccounting/internal/repository/account"
	budgetRepo "raccounting/internal/repository/budget"
	categoryRepo "raccounting/internal/repository/category"
	currencyRepo "raccounting/internal/repository/currency"
	sessionRepo "raccounting/internal/repository/session"
	tagRepo "raccounting/internal/repository/tag"
	transactionRepo "raccounting/internal/repository/transaction"
	userRepo "raccounting/internal/repository/user"
)

// repositories bundles every table-group repository built on top of storage. It exists so use-case
// wiring can take just the repositories it needs without Run growing a long, error-prone parameter
// list of its own.
type repositories struct {
	Users        *userRepo.Repository
	Sessions     *sessionRepo.Repository
	Accounts     *accountRepo.Repository
	Categories   *categoryRepo.Repository
	Currencies   *currencyRepo.Repository
	Transactions *transactionRepo.Repository
	Budgets      *budgetRepo.Repository
	Tags         *tagRepo.Repository
}

// newRepositories builds every repository on top of the same storage connection.
func newRepositories(store storage) repositories {
	return repositories{
		Users:        userRepo.New(store),
		Sessions:     sessionRepo.New(store),
		Accounts:     accountRepo.New(store),
		Categories:   categoryRepo.New(store),
		Currencies:   currencyRepo.New(store),
		Transactions: transactionRepo.New(store),
		Budgets:      budgetRepo.New(store),
		Tags:         tagRepo.New(store),
	}
}
