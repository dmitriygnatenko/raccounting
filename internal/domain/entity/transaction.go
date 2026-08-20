package entity

import (
	"encoding/json"
	"time"
)

// MaxMemoLength is the maximum accepted length for Transaction.Memo.
const MaxMemoLength = 1000

// DateLayout is the wire format for a transaction/transfer date ("YYYY-MM-DD") — no time-of-day,
// matching the frontend's <input type="date">.
const DateLayout = "2006-01-02"

type TransactionType uint8

const (
	TransactionTypeIncome TransactionType = iota + 1
	TransactionTypeExpense
	TransactionTypeTransfer
)

// Transaction is a single ledger entry: an expense, income, or one leg of a transfer between two
// accounts. Transfer legs share a TransferTransactionID pointing at each other and carry CategoryID
// nil. Amount is signed: negative for money leaving Account (expense, or a transfer's debit leg),
// positive for money arriving (income, or a transfer's credit leg).
type Transaction struct {
	ID                    uint64
	CategoryID            *uint64
	Type                  TransactionType
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
}

// IsTransfer reports whether t is one leg of a transfer.
func (t Transaction) IsTransfer() bool {
	return t.Type == TransactionTypeTransfer
}

// transactionJSON is Transaction's wire shape: Type as its bare numeric value (the frontend keeps
// its own App.TransactionType constants), amounts as decimal major-unit values, OperationAt as a
// bare date.
type transactionJSON struct {
	ID                    uint64    `json:"id"`
	Type                  uint8     `json:"type"`
	AccountID             uint64    `json:"accountId"`
	CategoryID            *uint64   `json:"categoryId,omitempty"`
	Currency              string    `json:"currency"`
	Amount                int64     `json:"amount"`
	TransferTransactionID *uint64   `json:"transferTransactionId,omitempty"`
	TransferCurrency      *string   `json:"transferCurrency,omitempty"`
	TransferAmount        *int64    `json:"transferAmount,omitempty"`
	TransferRate          *float64  `json:"transferRate,omitempty"`
	TransferAccountID     *uint64   `json:"transferAccountId,omitempty"`
	Memo                  string    `json:"memo"`
	Date                  string    `json:"date"`
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

func (t Transaction) MarshalJSON() ([]byte, error) {
	j := transactionJSON{
		ID:                    t.ID,
		Type:                  uint8(t.Type),
		AccountID:             t.AccountID,
		CategoryID:            t.CategoryID,
		Currency:              t.CurrencyCode,
		Amount:                t.Amount,
		TransferTransactionID: t.TransferTransactionID,
		TransferCurrency:      t.TransferCurrencyCode,
		TransferAccountID:     t.TransferAccountID,
		Memo:                  t.Memo,
		Date:                  t.OperationAt.Format(DateLayout),
		CreatedAt:             t.CreatedAt,
		UpdatedAt:             t.UpdatedAt,
	}

	if t.TransferAmount != nil {
		amount := *t.TransferAmount
		j.TransferAmount = &amount
	}

	if t.TransferRate != nil {
		rate := *t.TransferRate
		j.TransferRate = &rate
	}

	return json.Marshal(j)
}
