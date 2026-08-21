package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Transaction is the shape of a row in the transactions table. The Transfer* columns are only
// populated on a transfer leg (Type == entity.TransactionTypeTransfer): they mirror the paired
// leg's own id/currency/amount/rate/account, stored redundantly on each row at transfer creation so
// a read never needs to join back to the other leg. They stay NULL for a plain expense/income row.
type Transaction struct {
	ID                    uint64
	CategoryID            *uint64
	Type                  uint8
	AccountID             uint64
	CurrencyCode          string
	Amount                int64
	TransferTransactionID *uint64
	TransferCurrencyCode  *string
	TransferAmount        *int64
	TransferRate          *float64
	TransferAccountID     *uint64
	Memo                  string
	OperationAt           time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
	TagIDs                []uint64
}

// ToEntity converts the stored row into a domain entity.Transaction.
func (m Transaction) ToEntity() entity.Transaction {
	return entity.Transaction{
		ID:                    m.ID,
		CategoryID:            m.CategoryID,
		Type:                  entity.TransactionType(m.Type),
		AccountID:             m.AccountID,
		CurrencyCode:          m.CurrencyCode,
		Amount:                m.Amount,
		TransferTransactionID: m.TransferTransactionID,
		TransferCurrencyCode:  m.TransferCurrencyCode,
		TransferAmount:        m.TransferAmount,
		TransferRate:          m.TransferRate,
		TransferAccountID:     m.TransferAccountID,
		Memo:                  m.Memo,
		OperationAt:           m.OperationAt,
		CreatedAt:             m.CreatedAt,
		UpdatedAt:             m.UpdatedAt,
		TagIDs:                m.TagIDs,
	}
}

// TransactionCreateRequest bundles the parameters Storage.CreateTransactionWithBalance needs.
// Creating a transaction also adjusts its account's balance, atomically, in the same DB
// transaction. CurrencyCode is the account's currency at the time of the transaction (resolved by
// the use case, not supplied by the caller).
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

// TransactionUpdateRequest bundles the parameters Storage.UpdateTransactionWithBalance needs.
// Updating a transaction reverses its old balance effect and applies the new one, atomically, in
// the same DB transaction — even when AccountID changed.
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

// TransferCreateRequest bundles the parameters Storage.CreateTransferWithBalance needs.
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

// CreateTransferResult is the pair of transaction legs Storage.CreateTransferWithBalance creates.
type CreateTransferResult struct {
	LegFrom Transaction
	LegTo   Transaction
}

// TransactionListFilter narrows Storage.ListTransactionsFiltered to a page of transactions matching
// every non-nil/non-empty field. DateFrom/DateTo are inclusive, date-only bounds. Page is 1-based;
// PageSize is the number of rows per page.
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

// ListTransactionsFilteredResult is what Storage.ListTransactionsFiltered returns: the requested
// page, plus enough to paginate (TotalCount) and to show an accurate total for the whole filtered
// set rather than just the page (SumsByCurrency — the SUM(amount) of every matching row, keyed by
// currency code, since converting to one base currency requires exchange rates the storage layer
// doesn't have).
type ListTransactionsFilteredResult struct {
	Transactions   []Transaction
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
