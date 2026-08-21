package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"raccounting/internal/domain/entity"
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

// transactionColumnsQualified is transactionColumns with every column qualified by the `t` alias —
// used by ListTransactionsFiltered, whose query joins in `categories c` to support searching by
// category name.
const transactionColumnsQualified = `t.id, t.category_id, t.type, t.account_id, t.currency, t.amount,
	t.transfer_transaction_id, t.transfer_currency, t.transfer_amount, t.transfer_rate, t.transfer_account_id,
	t.memo, t.operation_at, t.created_at, t.updated_at`

// buildTransactionListWhere builds the SQL WHERE clause and its bound args for filter, shared by the
// count, per-currency sum, and paginated queries in ListTransactionsFiltered. Assumes the transactions
// table is aliased `t`, and — only when filter.Search is set — that a LEFT JOIN to `categories c` is
// also in scope.
func buildTransactionListWhere(filter model.TransactionListFilter) (string, []any) {
	var (
		conditions []string
		args       []any
	)

	if filter.DateFrom != nil {
		conditions = append(conditions, "t.operation_at >= ?")
		args = append(args, filter.DateFrom.Format(entity.DateLayout))
	}

	if filter.DateTo != nil {
		conditions = append(conditions, "t.operation_at <= ?")
		args = append(args, filter.DateTo.Format(entity.DateLayout))
	}

	if filter.AccountID != nil {
		conditions = append(conditions, "t.account_id = ?")
		args = append(args, *filter.AccountID)
	}

	if filter.CategoryID != nil {
		conditions = append(conditions, "t.category_id = ?")
		args = append(args, *filter.CategoryID)
	}

	if filter.Type != nil {
		conditions = append(conditions, "t.type = ?")
		args = append(args, uint8(*filter.Type))
	}

	if filter.TagID != nil {
		conditions = append(conditions,
			"EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id AND tt.tag_id = ?)",
		)
		args = append(args, *filter.TagID)
	}

	if filter.Search != "" {
		like := "%" + filter.Search + "%"
		conditions = append(conditions, "(t.memo LIKE ? OR c.name LIKE ?)")
		args = append(args, like, like)
	}

	if len(conditions) == 0 {
		return "", args
	}

	return " WHERE " + strings.Join(conditions, " AND "), args
}

// ListTransactionsFiltered returns one page of transactions matching filter, most recent operation
// first, alongside the total count and per-currency sums across every matching row (not just the
// page) — SumsByCurrency lets a caller show an accurate total for the whole filtered set without
// loading it all.
func (s *Storage) ListTransactionsFiltered(
	ctx context.Context, filter model.TransactionListFilter,
) (model.ListTransactionsFilteredResult, error) {
	from := "transactions t"
	if filter.Search != "" {
		from += " LEFT JOIN categories c ON c.id = t.category_id"
	}

	where, args := buildTransactionListWhere(filter)

	var total int
	if err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM `+from+where, args...,
	).Scan(&total); err != nil {
		return model.ListTransactionsFilteredResult{}, err
	}

	sums, err := s.transactionSumsByCurrency(ctx, from, where, args)
	if err != nil {
		return model.ListTransactionsFilteredResult{}, err
	}

	page, pageSize := filter.Page, filter.PageSize
	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 100
	}

	pageArgs := append(append([]any{}, args...), pageSize, (page-1)*pageSize)

	rows, err := s.DB.QueryContext(ctx,
		`SELECT `+transactionColumnsQualified+` FROM `+from+where+
			` ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`,
		pageArgs...,
	)
	if err != nil {
		return model.ListTransactionsFilteredResult{}, err
	}
	defer rows.Close()

	var transactions []model.Transaction

	for rows.Next() {
		m, err := scanTransaction(rows)
		if err != nil {
			return model.ListTransactionsFilteredResult{}, err
		}

		transactions = append(transactions, m)
	}

	if err = rows.Err(); err != nil {
		return model.ListTransactionsFilteredResult{}, err
	}

	if len(transactions) > 0 {
		ids := make([]uint64, len(transactions))
		for i, t := range transactions {
			ids[i] = t.ID
		}

		tagsByTransaction, err := s.listTransactionTagsFor(ctx, ids)
		if err != nil {
			return model.ListTransactionsFilteredResult{}, err
		}

		for i := range transactions {
			transactions[i].TagIDs = tagsByTransaction[transactions[i].ID]
		}
	}

	return model.ListTransactionsFilteredResult{
		Transactions:   transactions,
		TotalCount:     total,
		SumsByCurrency: sums,
	}, nil
}

// transactionSumsByCurrency runs the SUM(amount)...GROUP BY currency query behind
// ListTransactionsFiltered's SumsByCurrency, against the same FROM/WHERE/args its caller used for
// the count.
func (s *Storage) transactionSumsByCurrency(
	ctx context.Context, from, where string, args []any,
) (map[string]int64, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT t.currency, SUM(t.amount) FROM `+from+where+` GROUP BY t.currency`, args...,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sums := make(map[string]int64)

	for rows.Next() {
		var (
			currency string
			sum      int64
		)

		if err = rows.Scan(&currency, &sum); err != nil {
			return nil, err
		}

		sums[currency] = sum
	}

	return sums, rows.Err()
}

// listTransactionTagsFor returns tag ids for only the given transaction ids, keyed by transaction id
// — used by ListTransactionsFiltered so a page's tag lookup stays proportional to the page size, not
// the whole table (unlike listTransactionTags, which ListTransactions uses for its all-rows read).
func (s *Storage) listTransactionTagsFor(ctx context.Context, ids []uint64) (map[uint64][]uint64, error) {
	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))

	for i, id := range ids {
		placeholders[i] = "?"
		args[i] = id
	}

	rows, err := s.DB.QueryContext(ctx,
		`SELECT transaction_id, tag_id FROM transaction_tags WHERE transaction_id IN (`+
			strings.Join(placeholders, ",")+`)`,
		args...,
	)
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

// TransactionUsage returns the account/category ids referenced by at least one transaction — cheap,
// distinct-value queries bounded by account/category count rather than transaction count.
func (s *Storage) TransactionUsage(ctx context.Context) (model.TransactionUsage, error) {
	accountIDs, err := s.distinctTransactionUint64(ctx, `SELECT DISTINCT account_id FROM transactions`)
	if err != nil {
		return model.TransactionUsage{}, err
	}

	categoryIDs, err := s.distinctTransactionUint64(
		ctx, `SELECT DISTINCT category_id FROM transactions WHERE category_id IS NOT NULL`,
	)
	if err != nil {
		return model.TransactionUsage{}, err
	}

	return model.TransactionUsage{AccountIDs: accountIDs, CategoryIDs: categoryIDs}, nil
}

// distinctTransactionUint64 runs a single-column query returning distinct uint64 ids.
func (s *Storage) distinctTransactionUint64(ctx context.Context, query string) ([]uint64, error) {
	rows, err := s.DB.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []uint64

	for rows.Next() {
		var id uint64

		if err = rows.Scan(&id); err != nil {
			return nil, err
		}

		ids = append(ids, id)
	}

	return ids, rows.Err()
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
	ctx context.Context, req model.TransactionCreateRequest,
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
//
// Unlike the MySQL/Postgres adapters, the row lookup below doesn't use SELECT ... FOR UPDATE: SQLite
// doesn't support it, and the connection pool here is capped at one open connection (see
// storage.go), so this transaction already has exclusive access.
func (s *Storage) UpdateTransactionWithBalance(
	ctx context.Context, req model.TransactionUpdateRequest,
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
		`SELECT account_id, amount FROM transactions WHERE id = ?`, req.ID,
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
// atomically, in one DB transaction. found is false if no transaction with this id existed. See
// UpdateTransactionWithBalance for why this doesn't need SELECT ... FOR UPDATE.
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
		`SELECT account_id, amount FROM transactions WHERE id = ?`, id,
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
