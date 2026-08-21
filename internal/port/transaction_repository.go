package port

//go:generate go tool mockgen -source=transaction_repository.go -destination=mocks/transaction_repository_mock.go -package=mocks

import (
	"context"
	"time"

	"raccounting/internal/domain/entity"
)

// TransactionCreateRequest bundles the TransactionRepository.Create parameters. Creating a
// transaction also adjusts its account's balance, atomically, in the same DB transaction.
// CurrencyCode is the account's currency at the time of the transaction (resolved by the use case,
// not supplied by the caller).
type TransactionCreateRequest struct {
	AccountID    uint64
	CategoryID   *uint64
	Type         entity.TransactionType
	CurrencyCode string
	Amount       int64
	Memo         string
	OperationAt  time.Time
	TagIDs       []uint64
}

// TransactionUpdateRequest bundles the TransactionRepository.Update parameters. Updating a
// transaction reverses its old balance effect and applies the new one, atomically, in the same DB
// transaction — even when AccountID changed.
type TransactionUpdateRequest struct {
	ID           uint64
	AccountID    uint64
	CategoryID   *uint64
	Type         entity.TransactionType
	CurrencyCode string
	Amount       int64
	Memo         string
	OperationAt  time.Time
	TagIDs       []uint64
}

// TransferCreateRequest bundles the TransactionRepository.CreateTransfer parameters.
// Amount/CreditAmount are positive magnitudes in their own account's currency: the debit leg
// (FromAccountID) is stored as -Amount, the credit leg (ToAccountID) as +CreditAmount. Rate is
// CreditAmount/Amount.
type TransferCreateRequest struct {
	FromAccountID    uint64
	FromCurrencyCode string
	ToAccountID      uint64
	ToCurrencyCode   string
	Amount           int64
	CreditAmount     int64
	Rate             float64
	OperationAt      time.Time
	Memo             string
}

// TransferResult is the pair of transaction legs a transfer creates.
type TransferResult struct {
	LegFrom entity.Transaction
	LegTo   entity.Transaction
}

// TransactionListFilter narrows TransactionRepository.ListFiltered to a page of transactions
// matching every non-nil/non-empty field. DateFrom/DateTo are inclusive, date-only bounds. Page is
// 1-based; PageSize is the number of rows per page.
type TransactionListFilter struct {
	DateFrom   *time.Time
	DateTo     *time.Time
	AccountID  *uint64
	CategoryID *uint64
	TagID      *uint64
	Type       *entity.TransactionType
	Search     string
	Page       int
	PageSize   int
}

// TransactionListResult is what ListFiltered returns: the requested page, plus enough to paginate
// (TotalCount, TotalPages) and to show an accurate total for the whole filtered set rather than just
// the page (SumsByCurrency — the SUM(amount) of every matching row, keyed by currency code, since
// converting to one base currency requires exchange rates the repository doesn't have).
type TransactionListResult struct {
	Transactions   []entity.Transaction
	TotalCount     int
	SumsByCurrency map[string]int64
}

// TransactionUsage is the set of account/category ids referenced by at least one transaction — cheap
// to compute (bounded by account/category count, not transaction count) and used to gate "delete
// this account/category" UI without loading every transaction.
type TransactionUsage struct {
	AccountIDs  []uint64
	CategoryIDs []uint64
}

// TransactionRepository persists Transactions, keeping each transaction's account balance in sync
// as a side effect of Create/Update/Delete.
// Transfer legs (Type == entity.TransactionTypeTransfer) are managed through
// CreateTransfer/DeleteTransfer instead, not through Create/Update.
type TransactionRepository interface {
	List(ctx context.Context) ([]entity.Transaction, error)
	// ListFiltered returns one page of transactions matching filter, most recent operation first.
	ListFiltered(ctx context.Context, filter TransactionListFilter) (TransactionListResult, error)
	// Usage returns the account/category ids referenced by at least one transaction.
	Usage(ctx context.Context) (TransactionUsage, error)
	// FindByID returns a *domainerror.NotFoundError if no transaction with this id exists.
	FindByID(ctx context.Context, id uint64) (entity.Transaction, error)
	// Create returns a *domainerror.ConflictError if applying the transaction's balance effect would
	// take its account negative.
	Create(ctx context.Context, req TransactionCreateRequest) (entity.Transaction, error)
	// Update returns a *domainerror.NotFoundError if no transaction with this id exists, or a
	// *domainerror.ConflictError if applying the new balance effect would take its account negative.
	Update(ctx context.Context, req TransactionUpdateRequest) (entity.Transaction, error)
	// Delete returns a *domainerror.NotFoundError if no transaction with this id exists, or a
	// *domainerror.ConflictError if reversing its balance effect would take its account negative.
	Delete(ctx context.Context, id uint64) error
	// CreateTransfer inserts a transfer's two linked legs (both Type == entity.TransactionTypeTransfer,
	// sharing one another's id via TransferTransactionID) and adjusts both accounts' balances,
	// atomically. raccounting is single-user, so transfers aren't scoped to a user. Returns a
	// *domainerror.ConflictError if applying either leg's balance effect would take its account
	// negative.
	CreateTransfer(ctx context.Context, req TransferCreateRequest) (TransferResult, error)
	// DeleteTransfer removes both legs of the transfer that id belongs to and reverses their balance
	// effects. found is false if no transaction with this id exists. Returns a
	// *domainerror.ConflictError if reversing either leg's balance effect would take its account
	// negative.
	DeleteTransfer(ctx context.Context, id uint64) (found bool, err error)
}
