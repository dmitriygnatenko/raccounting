package mysql

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

	if err = rows.Err(); err != nil {
		return nil, err
	}

	tagsByTransaction, err := s.listTransactionTags(ctx)
	if err != nil {
		return nil, err
	}

	for i := range transactions {
		transactions[i].TagIDs = tagsByTransaction[transactions[i].ID]
	}

	return transactions, nil
}

// FindTransactionByID returns a transaction by id. sql.ErrNoRows if it doesn't exist.
func (s *Storage) FindTransactionByID(ctx context.Context, id uint64) (model.Transaction, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+transactionColumns+` FROM transactions WHERE id = ?`, id)

	m, err := scanTransaction(row)
	if err != nil {
		return model.Transaction{}, err
	}

	m.TagIDs, err = s.findTransactionTagIDs(ctx, id)

	return m, err
}

// listTransactionTags returns every transaction's tag ids in one query, keyed by transaction id —
// used by ListTransactions to avoid an N+1 query per row.
func (s *Storage) listTransactionTags(ctx context.Context) (map[uint64][]uint64, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT transaction_id, tag_id FROM transaction_tags`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	byTransaction := make(map[uint64][]uint64)

	for rows.Next() {
		var transactionID, tagID uint64

		if err = rows.Scan(&transactionID, &tagID); err != nil {
			return nil, err
		}

		byTransaction[transactionID] = append(byTransaction[transactionID], tagID)
	}

	return byTransaction, rows.Err()
}

// findTransactionTagIDs returns the tag ids attached to a single transaction.
func (s *Storage) findTransactionTagIDs(ctx context.Context, transactionID uint64) ([]uint64, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT tag_id FROM transaction_tags WHERE transaction_id = ?`, transactionID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tagIDs []uint64

	for rows.Next() {
		var tagID uint64

		if err = rows.Scan(&tagID); err != nil {
			return nil, err
		}

		tagIDs = append(tagIDs, tagID)
	}

	return tagIDs, rows.Err()
}

// setTransactionTags replaces a transaction's tag associations with tagIDs, inside tx.
func setTransactionTags(ctx context.Context, tx *sql.Tx, transactionID uint64, tagIDs []uint64) error {
	if _, err := tx.ExecContext(ctx,
		`DELETE FROM transaction_tags WHERE transaction_id = ?`, transactionID,
	); err != nil {
		return err
	}

	for _, tagID := range tagIDs {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)`, transactionID, tagID,
		); err != nil {
			return err
		}
	}

	return nil
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

	res, err := tx.ExecContext(ctx,
		`INSERT INTO transactions (category_id, type, account_id, currency, amount, memo, operation_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt,
	)
	if err != nil {
		return model.Transaction{}, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		return model.Transaction{}, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		req.Amount, req.AccountID,
	); err != nil {
		return model.Transaction{}, wrapInsufficientBalance(err)
	}

	if err = setTransactionTags(ctx, tx, uint64(id), req.TagIDs); err != nil {
		return model.Transaction{}, err
	}

	if err = tx.Commit(); err != nil {
		return model.Transaction{}, err
	}

	return model.Transaction{
		ID:           uint64(id),
		CategoryID:   req.CategoryID,
		Type:         uint8(req.Type),
		AccountID:    req.AccountID,
		CurrencyCode: req.CurrencyCode,
		Amount:       req.Amount,
		Memo:         req.Memo,
		OperationAt:  req.OperationAt,
		TagIDs:       req.TagIDs,
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
		`SELECT account_id, amount FROM transactions WHERE id = ? FOR UPDATE`, req.ID,
	).Scan(&oldAccountID, &oldAmount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return model.Transaction{}, false, nil
		}

		return model.Transaction{}, false, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		oldAmount, oldAccountID,
	); err != nil {
		return model.Transaction{}, false, wrapInsufficientBalance(err)
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE transactions
		 SET account_id = ?, category_id = ?, type = ?, currency = ?, amount = ?, memo = ?,
		     operation_at = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`,
		req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
		req.OperationAt, req.ID,
	); err != nil {
		return model.Transaction{}, false, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		req.Amount, req.AccountID,
	); err != nil {
		return model.Transaction{}, false, wrapInsufficientBalance(err)
	}

	if err = setTransactionTags(ctx, tx, req.ID, req.TagIDs); err != nil {
		return model.Transaction{}, false, err
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
		TagIDs:       req.TagIDs,
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
		`SELECT account_id, amount FROM transactions WHERE id = ? FOR UPDATE`, id,
	).Scan(&accountID, &amount)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	if _, err = tx.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, id); err != nil {
		return false, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		amount, accountID,
	); err != nil {
		return false, wrapInsufficientBalance(err)
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}
