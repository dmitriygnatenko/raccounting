// Package transaction implements port.TransactionRepository on top of the transactions table. Every
// write here also adjusts the affected account's balance, atomically, inside the same DB
// transaction — see the DB adapters' *WithBalance methods. Transfers (two linked rows pointing at
// each other's id via transfer_transaction_id) are created and deleted here too, via
// CreateTransfer/DeleteTransfer.
package transaction

import (
	"context"
	"database/sql"
	"errors"

	"raccounting/internal/domain/entity"
	domainerror "raccounting/internal/domain/error"
	"raccounting/internal/port"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

//go:generate go tool mockgen -source=repository.go -destination=mocks/storage_mock.go -package=mocks

// Storage is the slice of the DB adapter this repository uses — the transactions table, plus the
// balance side effect on accounts.
type Storage interface {
	ListTransactions(ctx context.Context) ([]model.Transaction, error)
	// ListTransactionsFiltered returns one page of transactions matching filter, most recent
	// operation first, alongside the total count and per-currency sums across every matching row
	// (not just the page).
	ListTransactionsFiltered(
		ctx context.Context,
		filter model.TransactionListFilter,
	) (model.ListTransactionsFilteredResult, error)
	// TransactionUsage returns the account/category ids referenced by at least one transaction.
	TransactionUsage(ctx context.Context) (model.TransactionUsage, error)
	// FindTransactionByID returns sql.ErrNoRows when no transaction with this id exists.
	FindTransactionByID(ctx context.Context, id uint64) (model.Transaction, error)
	// CreateTransactionWithBalance inserts a transaction row and adjusts its account's balance,
	// atomically, in one DB transaction. An overdraw comes back wrapped in
	// storageError.InsufficientBalanceError.
	CreateTransactionWithBalance(
		ctx context.Context,
		req model.TransactionCreateRequest,
	) (model.Transaction, error)
	// UpdateTransactionWithBalance reverses the transaction's old balance effect and applies the new
	// one — even across an account change — atomically, in one DB transaction. found is false if no
	// transaction with this id exists. An overdraw comes back wrapped in
	// storageError.InsufficientBalanceError.
	UpdateTransactionWithBalance(
		ctx context.Context,
		req model.TransactionUpdateRequest,
	) (row model.Transaction, found bool, err error)
	// DeleteTransactionWithBalance removes a transaction row and reverses its balance effect,
	// atomically, in one DB transaction. found is false if no transaction with this id existed. An
	// overdraw comes back wrapped in storageError.InsufficientBalanceError.
	DeleteTransactionWithBalance(ctx context.Context, id uint64) (found bool, err error)
	// CreateTransferWithBalance inserts both transfer legs and adjusts both accounts' balances,
	// atomically, in one DB transaction. An overdraw on either leg comes back wrapped in
	// storageError.InsufficientBalanceError.
	CreateTransferWithBalance(
		ctx context.Context,
		req model.TransferCreateRequest,
	) (model.CreateTransferResult, error)
	// DeleteTransferWithBalance removes both legs of the transfer that id belongs to and reverses
	// their balance effects, atomically, in one DB transaction. found is false if no transaction
	// with this id existed. An overdraw on either leg comes back wrapped in
	// storageError.InsufficientBalanceError.
	DeleteTransferWithBalance(ctx context.Context, id uint64) (found bool, err error)
}

// Repository implements port.TransactionRepository.
type Repository struct {
	storage Storage
}

// New builds a Repository against s.
func New(s Storage) *Repository {
	return &Repository{storage: s}
}

// List returns every transaction.
func (r *Repository) List(ctx context.Context) ([]entity.Transaction, error) {
	rows, err := r.storage.ListTransactions(ctx)
	if err != nil {
		return nil, err
	}

	transactions := make([]entity.Transaction, len(rows))
	for i, row := range rows {
		transactions[i] = row.ToEntity()
	}

	return transactions, nil
}

// ListFiltered returns one page of transactions matching filter.
func (r *Repository) ListFiltered(
	ctx context.Context, filter port.TransactionListFilter,
) (port.TransactionListResult, error) {
	res, err := r.storage.ListTransactionsFiltered(ctx, model.TransactionListFilter{
		DateFrom:   filter.DateFrom,
		DateTo:     filter.DateTo,
		AccountID:  filter.AccountID,
		CategoryID: filter.CategoryID,
		TagID:      filter.TagID,
		Type:       filter.Type,
		Search:     filter.Search,
		Page:       filter.Page,
		PageSize:   filter.PageSize,
	})
	if err != nil {
		return port.TransactionListResult{}, err
	}

	transactions := make([]entity.Transaction, len(res.Transactions))
	for i, row := range res.Transactions {
		transactions[i] = row.ToEntity()
	}

	return port.TransactionListResult{
		Transactions:   transactions,
		TotalCount:     res.TotalCount,
		SumsByCurrency: res.SumsByCurrency,
	}, nil
}

// Usage returns the account/category ids referenced by at least one transaction.
func (r *Repository) Usage(ctx context.Context) (port.TransactionUsage, error) {
	res, err := r.storage.TransactionUsage(ctx)
	if err != nil {
		return port.TransactionUsage{}, err
	}

	return port.TransactionUsage{
		AccountIDs:  res.AccountIDs,
		CategoryIDs: res.CategoryIDs,
	}, nil
}

// FindByID looks up a transaction by id. An unknown id is reported as a message-less
// *domainerror.NotFoundError — the use case supplies the message.
func (r *Repository) FindByID(ctx context.Context, id uint64) (entity.Transaction, error) {
	row, err := r.storage.FindTransactionByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return entity.Transaction{}, &domainerror.NotFoundError{}
		}

		return entity.Transaction{}, err
	}

	return row.ToEntity(), nil
}

// Create inserts a new transaction and adjusts its account's balance. A negative resulting balance
// is reported as a message-less *domainerror.ConflictError — the use case supplies the message.
func (r *Repository) Create(
	ctx context.Context, req port.TransactionCreateRequest,
) (entity.Transaction, error) {
	row, err := r.storage.CreateTransactionWithBalance(ctx, model.TransactionCreateRequest{
		AccountID:    req.AccountID,
		CategoryID:   req.CategoryID,
		Type:         req.Type,
		CurrencyCode: req.CurrencyCode,
		Amount:       req.Amount,
		Memo:         req.Memo,
		OperationAt:  req.OperationAt,
		TagIDs:       req.TagIDs,
	})
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return entity.Transaction{}, &domainerror.ConflictError{}
		}

		return entity.Transaction{}, err
	}

	return row.ToEntity(), nil
}

// Update changes an existing transaction, reversing its old balance effect and applying the new
// one. An unknown id is reported as a message-less *domainerror.NotFoundError, and a negative
// resulting balance as a message-less *domainerror.ConflictError — the use case supplies the
// message either way.
func (r *Repository) Update(
	ctx context.Context, req port.TransactionUpdateRequest,
) (entity.Transaction, error) {
	row, found, err := r.storage.UpdateTransactionWithBalance(ctx, model.TransactionUpdateRequest{
		ID:           req.ID,
		AccountID:    req.AccountID,
		CategoryID:   req.CategoryID,
		Type:         req.Type,
		CurrencyCode: req.CurrencyCode,
		Amount:       req.Amount,
		Memo:         req.Memo,
		OperationAt:  req.OperationAt,
		TagIDs:       req.TagIDs,
	})
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return entity.Transaction{}, &domainerror.ConflictError{}
		}

		return entity.Transaction{}, err
	}

	if !found {
		return entity.Transaction{}, &domainerror.NotFoundError{}
	}

	return row.ToEntity(), nil
}

// Delete removes a transaction and reverses its balance effect. An unknown id is reported as a
// message-less *domainerror.NotFoundError, and a negative resulting balance as a message-less
// *domainerror.ConflictError — the use case supplies the message either way.
func (r *Repository) Delete(ctx context.Context, id uint64) error {
	found, err := r.storage.DeleteTransactionWithBalance(ctx, id)
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return &domainerror.ConflictError{}
		}

		return err
	}

	if !found {
		return &domainerror.NotFoundError{}
	}

	return nil
}

// CreateTransfer inserts a new transfer's two legs and adjusts both accounts' balances. A negative
// resulting balance on either leg is reported as a message-less *domainerror.ConflictError — the
// use case supplies the message.
func (r *Repository) CreateTransfer(
	ctx context.Context, req port.TransferCreateRequest,
) (port.TransferResult, error) {
	res, err := r.storage.CreateTransferWithBalance(ctx, model.TransferCreateRequest{
		FromAccountID:    req.FromAccountID,
		FromCurrencyCode: req.FromCurrencyCode,
		ToAccountID:      req.ToAccountID,
		ToCurrencyCode:   req.ToCurrencyCode,
		Amount:           req.Amount,
		CreditAmount:     req.CreditAmount,
		Rate:             req.Rate,
		OperationAt:      req.OperationAt,
		Memo:             req.Memo,
	})
	if err != nil {
		if errors.Is(err, storageError.InsufficientBalanceError) {
			return port.TransferResult{}, &domainerror.ConflictError{}
		}

		return port.TransferResult{}, err
	}

	return port.TransferResult{
		LegFrom: res.LegFrom.ToEntity(),
		LegTo:   res.LegTo.ToEntity(),
	}, nil
}

// DeleteTransfer removes both legs of a transfer and reverses their balance effects. A negative
// resulting balance on either leg is reported as a message-less *domainerror.ConflictError — the
// use case supplies the message.
func (r *Repository) DeleteTransfer(ctx context.Context, id uint64) (bool, error) {
	found, err := r.storage.DeleteTransferWithBalance(ctx, id)
	if err != nil && errors.Is(err, storageError.InsufficientBalanceError) {
		return false, &domainerror.ConflictError{}
	}

	return found, err
}
