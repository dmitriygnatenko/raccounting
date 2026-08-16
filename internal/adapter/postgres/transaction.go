package postgres

import (
	"context"
	"database/sql"
	"errors"

	"raccounting/internal/port"
	"raccounting/internal/storage/model"
)

// transactionScanner is satisfied by both *sql.Row and *sql.Rows.
type transactionScanner interface {
	Scan(dest ...any) error
}

// scanTransaction reads the transaction column list, in the order every transaction query below
// selects it, translating SQL NULLs into the model's pointer fields.
func scanTransaction(row transactionScanner) (model.Transaction, error) {
	var (
		m                     model.Transaction
		categoryID            sql.NullInt64
		transferTransactionID sql.NullInt64
		transferCurrencyCode  sql.NullString
		transferAmount        sql.NullInt64
		transferRate          sql.NullFloat64
		transferAccountID     sql.NullInt64
	)

	err := row.Scan(
		&m.ID, &categoryID, &m.Type, &m.AccountID, &m.CurrencyCode, &m.Amount,
		&transferTransactionID, &transferCurrencyCode, &transferAmount, &transferRate, &transferAccountID,
		&m.Memo, &m.OperationAt, &m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return model.Transaction{}, err
	}

	if categoryID.Valid {
		v := uint64(categoryID.Int64)
		m.CategoryID = &v
	}

	if transferTransactionID.Valid {
		v := uint64(transferTransactionID.Int64)
		m.TransferTransactionID = &v
	}

	if transferCurrencyCode.Valid {
		v := transferCurrencyCode.String
		m.TransferCurrencyCode = &v
	}

	if transferAmount.Valid {
		v := transferAmount.Int64
		m.TransferAmount = &v
	}

	if transferRate.Valid {
		v := transferRate.Float64
		m.TransferRate = &v
	}

	if transferAccountID.Valid {
		v := uint64(transferAccountID.Int64)
		m.TransferAccountID = &v
	}

	return m, nil
}

const transactionColumns = `id, category_id, type, account_id, currency, amount,
	transfer_transaction_id, transfer_currency, transfer_amount, transfer_rate, transfer_account_id,
	memo, operation_at, created_at, updated_at`

// ListTransactions returns every transaction, most recent operation first.
func (s *Storage) ListTransactions(ctx context.Context) ([]model.Transaction, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+transactionColumns+` FROM transactions ORDER BY operation_at DESC, id DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var transactions []model.Transaction

	for rows.Next() {
		m, err := scanTransaction(rows)
		if err != nil {
			return nil, err
		}

		transactions = append(transactions, m)
	}

	return transactions, rows.Err()
}

// FindTransactionByID returns a transaction by id. sql.ErrNoRows if it doesn't exist.
func (s *Storage) FindTransactionByID(ctx context.Context, id uint64) (model.Transaction, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE id = $1`, id)

	return scanTransaction(row)
}

// CreateTransactionWithBalance inserts a transaction row and adjusts its account's balance,
// atomically, in one DB transaction.
func (s *Storage) CreateTransactionWithBalance(
	ctx context.Context, req port.TransactionCreateRequest,
) (model.Transaction, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return model.Transaction{}, err
	}
	defer func() { _ = tx.Rollback() }()

	var id uint64

	err = tx.QueryRowContext(ctx,
		`INSERT INTO transactions (category_id, type, account_id, currency, amount, memo, operation_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt,
	).Scan(&id)
	if err != nil {
		return model.Transaction{}, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		req.Amount, req.AccountID,
	); err != nil {
		return model.Transaction{}, wrapInsufficientBalance(err)
	}

	if err = tx.Commit(); err != nil {
		return model.Transaction{}, err
	}

	return model.Transaction{
		ID:           id,
		CategoryID:   req.CategoryID,
		Type:         uint8(req.Type),
		AccountID:    req.AccountID,
		CurrencyCode: req.CurrencyCode,
		Amount:       req.Amount,
		Memo:         req.Memo,
		OperationAt:  req.OperationAt,
	}, nil
}

// UpdateTransactionWithBalance reverses the transaction's old balance effect and applies the new one
// — even across an account change — atomically, in one DB transaction. found is false if no
// transaction with this id exists.
func (s *Storage) UpdateTransactionWithBalance(
	ctx context.Context, req port.TransactionUpdateRequest,
) (model.Transaction, bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return model.Transaction{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		oldAccountID uint64
		oldAmount    int64
	)

	err = tx.QueryRowContext(ctx,
		`SELECT account_id, amount FROM transactions WHERE id = $1 FOR UPDATE`, req.ID,
	).Scan(&oldAccountID, &oldAmount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Transaction{}, false, nil
		}

		return model.Transaction{}, false, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		oldAmount, oldAccountID,
	); err != nil {
		return model.Transaction{}, false, wrapInsufficientBalance(err)
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE transactions
		 SET account_id = $1, category_id = $2, type = $3, currency = $4, amount = $5, memo = $6,
		     operation_at = $7, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $8`,
		req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
		req.OperationAt, req.ID,
	); err != nil {
		return model.Transaction{}, false, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		req.Amount, req.AccountID,
	); err != nil {
		return model.Transaction{}, false, wrapInsufficientBalance(err)
	}

	if err = tx.Commit(); err != nil {
		return model.Transaction{}, false, err
	}

	return model.Transaction{
		ID:           req.ID,
		CategoryID:   req.CategoryID,
		Type:         uint8(req.Type),
		AccountID:    req.AccountID,
		CurrencyCode: req.CurrencyCode,
		Amount:       req.Amount,
		Memo:         req.Memo,
		OperationAt:  req.OperationAt,
	}, true, nil
}

// DeleteTransactionWithBalance removes a transaction row and reverses its balance effect,
// atomically, in one DB transaction. found is false if no transaction with this id existed.
func (s *Storage) DeleteTransactionWithBalance(ctx context.Context, id uint64) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		accountID uint64
		amount    int64
	)

	err = tx.QueryRowContext(ctx,
		`SELECT account_id, amount FROM transactions WHERE id = $1 FOR UPDATE`, id,
	).Scan(&accountID, &amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM transactions WHERE id = $1`, id); err != nil {
		return false, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`,
		amount, accountID,
	); err != nil {
		return false, wrapInsufficientBalance(err)
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}
