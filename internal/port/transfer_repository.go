package port

import (
	"context"
	"time"

	"raccounting/internal/domain/entity"
)

// TransferCreateRequest bundles the TransferRepository.Create parameters. Amount/CreditAmount are
// positive magnitudes in their own account's currency: the debit leg (FromAccountID) is stored as
// -Amount, the credit leg (ToAccountID) as +CreditAmount. Rate is CreditAmount/Amount.
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

// TransferRepository creates and deletes transfers: two linked Transaction rows (both
// Type == entity.TransactionTypeTransfer) sharing one another's id via TransferTransactionID, with
// both accounts' balances kept in sync atomically alongside them. raccounting is single-user, so
// transfers aren't scoped to a user.
type TransferRepository interface {
	Create(ctx context.Context, req TransferCreateRequest) (TransferResult, error)
	// Delete removes both legs of the transfer that id belongs to and reverses their balance
	// effects. found is false if no transaction with this id exists.
	Delete(ctx context.Context, id uint64) (found bool, err error)
}
