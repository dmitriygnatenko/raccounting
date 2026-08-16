package model

import (
	"time"

	"raccounting/internal/domain/entity"
)

// Transaction is the shape of a row in the transactions table.
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
	}
}
