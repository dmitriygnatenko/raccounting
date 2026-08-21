package mysql

import (
	"context"
	"database/sql"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

func fakeTransactionRow() model.Transaction {
	categoryID := fakeID()

	return model.Transaction{
		ID:           fakeID(),
		CategoryID:   &categoryID,
		Type:         uint8(entity.TransactionTypeExpense),
		AccountID:    fakeID(),
		CurrencyCode: fakeCode(),
		Amount:       fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeTime(),
		CreatedAt:    fakeTime(),
		UpdatedAt:    fakeTime(),
	}
}

// addTransactionRow appends m in the exact column order transactionColumns/transactionColumnsQualified
// select, including the transfer-leg columns, which are NULL for a plain expense/income row.
func addTransactionRow(rows *sqlmock.Rows, m model.Transaction) *sqlmock.Rows {
	var categoryID, transferTxID, transferAccountID any
	var transferCurrency any
	var transferAmount, transferRate any

	if m.CategoryID != nil {
		categoryID = *m.CategoryID
	}

	if m.TransferTransactionID != nil {
		transferTxID = *m.TransferTransactionID
	}

	if m.TransferCurrencyCode != nil {
		transferCurrency = *m.TransferCurrencyCode
	}

	if m.TransferAmount != nil {
		transferAmount = *m.TransferAmount
	}

	if m.TransferRate != nil {
		transferRate = *m.TransferRate
	}

	if m.TransferAccountID != nil {
		transferAccountID = *m.TransferAccountID
	}

	return rows.AddRow(
		m.ID, categoryID, m.Type, m.AccountID, m.CurrencyCode, m.Amount,
		transferTxID, transferCurrency, transferAmount, transferRate, transferAccountID,
		m.Memo, m.OperationAt, m.CreatedAt, m.UpdatedAt,
	)
}

var transactionRowColumns = []string{
	"id", "category_id", "type", "account_id", "currency", "amount",
	"transfer_transaction_id", "transfer_currency", "transfer_amount", "transfer_rate", "transfer_account_id",
	"memo", "operation_at", "created_at", "updated_at",
}

// requireTransactionEqual compares every field scanTransaction populates, ignoring TagIDs — callers
// attach those separately once the row scan itself is confirmed correct.
func requireTransactionEqual(t *testing.T, want, got model.Transaction) {
	t.Helper()

	got.TagIDs = nil
	want.TagIDs = nil
	require.Equal(t, want, got)
}

// TestListTransactions covers the full listing plus its N+1-avoiding tag lookup.
func TestListTransactions(t *testing.T) {
	t.Parallel()

	listQuery := `SELECT ` + transactionColumns + ` FROM transactions ORDER BY operation_at DESC, id DESC`
	tagsQuery := `SELECT transaction_id, tag_id FROM transaction_tags`

	t.Run("an empty result yields no rows, no tag lookup needed but still run", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(listQuery).WillReturnRows(sqlmock.NewRows(transactionRowColumns))
		mock.ExpectQuery(tagsQuery).WillReturnRows(sqlmock.NewRows([]string{"transaction_id", "tag_id"}))

		got, err := s.ListTransactions(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("rows come back with their tags attached", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		tx1, tx2 := fakeTransactionRow(), fakeTransactionRow()
		tag1, tag2 := fakeID(), fakeID()

		rows := sqlmock.NewRows(transactionRowColumns)
		addTransactionRow(rows, tx1)
		addTransactionRow(rows, tx2)
		mock.ExpectQuery(listQuery).WillReturnRows(rows)

		mock.ExpectQuery(tagsQuery).WillReturnRows(
			sqlmock.NewRows([]string{"transaction_id", "tag_id"}).
				AddRow(tx1.ID, tag1).
				AddRow(tx1.ID, tag2),
		)

		got, err := s.ListTransactions(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 2)
		requireTransactionEqual(t, tx1, got[0])
		requireTransactionEqual(t, tx2, got[1])
		require.Equal(t, []uint64{tag1, tag2}, got[0].TagIDs)
		require.Empty(t, got[1].TagIDs)
	})

	t.Run("a driver error on the listing query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(listQuery).WillReturnError(errStub)

		_, err := s.ListTransactions(context.Background())
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the tags lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(listQuery).WillReturnRows(sqlmock.NewRows(transactionRowColumns))
		mock.ExpectQuery(tagsQuery).WillReturnError(errStub)

		_, err := s.ListTransactions(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestBuildTransactionListWhere is a direct unit test of the pure WHERE-clause builder: every filter
// field alone, then combined, since ListTransactionsFiltered's queries all depend on getting this
// exactly right.
func TestBuildTransactionListWhere(t *testing.T) {
	t.Parallel()

	dateFrom := fakeTime()
	dateTo := fakeTime()
	accountID := fakeID()
	categoryID := fakeID()
	tagID := fakeID()
	txType := entity.TransactionTypeExpense

	tests := []struct {
		name       string
		filter     model.TransactionListFilter
		wantWhere  string
		wantArgLen int
	}{
		{
			name:       "no filter yields no WHERE clause",
			filter:     model.TransactionListFilter{},
			wantWhere:  "",
			wantArgLen: 0,
		},
		{
			name:       "DateFrom alone",
			filter:     model.TransactionListFilter{DateFrom: &dateFrom},
			wantWhere:  " WHERE t.operation_at >= ?",
			wantArgLen: 1,
		},
		{
			name:       "DateTo alone",
			filter:     model.TransactionListFilter{DateTo: &dateTo},
			wantWhere:  " WHERE t.operation_at <= ?",
			wantArgLen: 1,
		},
		{
			name:       "AccountID alone",
			filter:     model.TransactionListFilter{AccountID: &accountID},
			wantWhere:  " WHERE t.account_id = ?",
			wantArgLen: 1,
		},
		{
			name:       "CategoryID alone",
			filter:     model.TransactionListFilter{CategoryID: &categoryID},
			wantWhere:  " WHERE t.category_id = ?",
			wantArgLen: 1,
		},
		{
			name:       "Type alone",
			filter:     model.TransactionListFilter{Type: &txType},
			wantWhere:  " WHERE t.type = ?",
			wantArgLen: 1,
		},
		{
			name:       "TagID alone",
			filter:     model.TransactionListFilter{TagID: &tagID},
			wantWhere:  " WHERE EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id AND tt.tag_id = ?)",
			wantArgLen: 1,
		},
		{
			name:       "Search alone binds two LIKE args",
			filter:     model.TransactionListFilter{Search: "coffee"},
			wantWhere:  " WHERE (t.memo LIKE ? OR c.name LIKE ?)",
			wantArgLen: 2,
		},
		{
			name: "every filter combined, joined with AND",
			filter: model.TransactionListFilter{
				DateFrom: &dateFrom, DateTo: &dateTo, AccountID: &accountID, CategoryID: &categoryID,
				TagID: &tagID, Type: &txType, Search: "coffee",
			},
			wantWhere: " WHERE t.operation_at >= ? AND t.operation_at <= ? AND t.account_id = ? AND t.category_id = ?" +
				" AND t.type = ? AND EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id AND tt.tag_id = ?)" +
				" AND (t.memo LIKE ? OR c.name LIKE ?)",
			wantArgLen: 8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			where, args := buildTransactionListWhere(tt.filter)
			require.Equal(t, tt.wantWhere, where)
			require.Len(t, args, tt.wantArgLen)
		})
	}
}

// TestListTransactionsFiltered covers the paginated, filtered listing: count, per-currency sums, and
// the page itself, plus the pagination defaults and the Search-driven JOIN to categories.
func TestListTransactionsFiltered(t *testing.T) {
	t.Parallel()

	countQuery := `SELECT COUNT(*) FROM transactions t`
	sumsQuery := `SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`
	pageQuery := `SELECT ` + transactionColumnsQualified +
		` FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`
	tagsForQuery := `SELECT transaction_id, tag_id FROM transaction_tags WHERE transaction_id IN (?)`

	t.Run("no filter, default pagination, no rows means no tag lookup", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(sumsQuery).WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(pageQuery).WithArgs(100, 0).WillReturnRows(sqlmock.NewRows(transactionRowColumns))

		got, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.NoError(t, err)
		require.Empty(t, got.Transactions)
		require.Equal(t, 0, got.TotalCount)
		require.Empty(t, got.SumsByCurrency)
	})

	t.Run("rows on the page trigger a scoped tag lookup", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		tx := fakeTransactionRow()
		tag := fakeID()

		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(sumsQuery).WillReturnRows(
			sqlmock.NewRows([]string{"currency", "sum"}).AddRow(tx.CurrencyCode, tx.Amount),
		)

		rows := sqlmock.NewRows(transactionRowColumns)
		addTransactionRow(rows, tx)
		mock.ExpectQuery(pageQuery).WithArgs(100, 0).WillReturnRows(rows)

		mock.ExpectQuery(tagsForQuery).WithArgs(tx.ID).WillReturnRows(
			sqlmock.NewRows([]string{"transaction_id", "tag_id"}).AddRow(tx.ID, tag),
		)

		got, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.NoError(t, err)
		require.Len(t, got.Transactions, 1)
		require.Equal(t, 1, got.TotalCount)
		require.Equal(t, map[string]int64{tx.CurrencyCode: tx.Amount}, got.SumsByCurrency)
		require.Equal(t, []uint64{tag}, got.Transactions[0].TagIDs)
	})

	t.Run("AccountID and Search combine to add the categories JOIN and both LIKE args", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		accountID := fakeID()
		search := "coffee"

		from := "transactions t LEFT JOIN categories c ON c.id = t.category_id"
		where := " WHERE t.account_id = ? AND (t.memo LIKE ? OR c.name LIKE ?)"
		like := "%" + search + "%"

		mock.ExpectQuery(`SELECT COUNT(*) FROM `+from+where).
			WithArgs(accountID, like, like).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM `+from+where+` GROUP BY t.currency`).
			WithArgs(accountID, like, like).WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(`SELECT `+transactionColumnsQualified+` FROM `+from+where+
			` ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`).
			WithArgs(accountID, like, like, 100, 0).WillReturnRows(sqlmock.NewRows(transactionRowColumns))

		got, err := s.ListTransactionsFiltered(
			context.Background(), model.TransactionListFilter{AccountID: &accountID, Search: search},
		)
		require.NoError(t, err)
		require.Empty(t, got.Transactions)
	})

	t.Run("Page < 1 and PageSize < 1 fall back to page 1, size 100", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(sumsQuery).WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(pageQuery).WithArgs(100, 0).WillReturnRows(sqlmock.NewRows(transactionRowColumns))

		_, err := s.ListTransactionsFiltered(
			context.Background(), model.TransactionListFilter{Page: -1, PageSize: 0},
		)
		require.NoError(t, err)
	})

	t.Run("Page 3 with PageSize 20 offsets by 40", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(sumsQuery).WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(pageQuery).WithArgs(20, 40).WillReturnRows(sqlmock.NewRows(transactionRowColumns))

		_, err := s.ListTransactionsFiltered(
			context.Background(), model.TransactionListFilter{Page: 3, PageSize: 20},
		)
		require.NoError(t, err)
	})

	t.Run("a driver error on the count query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(countQuery).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the sums query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(sumsQuery).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the page query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(sumsQuery).WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(pageQuery).WithArgs(100, 0).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the scoped tags lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		tx := fakeTransactionRow()

		mock.ExpectQuery(countQuery).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(sumsQuery).WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))

		rows := sqlmock.NewRows(transactionRowColumns)
		addTransactionRow(rows, tx)
		mock.ExpectQuery(pageQuery).WithArgs(100, 0).WillReturnRows(rows)

		mock.ExpectQuery(tagsForQuery).WithArgs(tx.ID).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})
}

// TestTransactionUsage covers the two distinct-value queries gating "delete this account/category".
func TestTransactionUsage(t *testing.T) {
	t.Parallel()

	accountQuery := `SELECT DISTINCT account_id FROM transactions`
	categoryQuery := `SELECT DISTINCT category_id FROM transactions WHERE category_id IS NOT NULL`

	t.Run("returns both distinct id sets", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		acct1, acct2 := fakeID(), fakeID()
		cat1 := fakeID()

		mock.ExpectQuery(accountQuery).WillReturnRows(
			sqlmock.NewRows([]string{"account_id"}).AddRow(acct1).AddRow(acct2),
		)
		mock.ExpectQuery(categoryQuery).WillReturnRows(sqlmock.NewRows([]string{"category_id"}).AddRow(cat1))

		got, err := s.TransactionUsage(context.Background())
		require.NoError(t, err)
		require.Equal(t, model.TransactionUsage{AccountIDs: []uint64{acct1, acct2}, CategoryIDs: []uint64{cat1}}, got)
	})

	t.Run("a driver error on the account query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(accountQuery).WillReturnError(errStub)

		_, err := s.TransactionUsage(context.Background())
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the category query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(accountQuery).WillReturnRows(sqlmock.NewRows([]string{"account_id"}))
		mock.ExpectQuery(categoryQuery).WillReturnError(errStub)

		_, err := s.TransactionUsage(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindTransactionByID covers the single-row lookup plus its per-transaction tag id fetch.
func TestFindTransactionByID(t *testing.T) {
	t.Parallel()

	rowQuery := `SELECT ` + transactionColumns + ` FROM transactions WHERE id = ?`
	tagsQuery := `SELECT tag_id FROM transaction_tags WHERE transaction_id = ?`

	tx := fakeTransactionRow()
	tag := fakeID()

	t.Run("finds the row and attaches its tags", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		rows := sqlmock.NewRows(transactionRowColumns)
		addTransactionRow(rows, tx)
		mock.ExpectQuery(rowQuery).WithArgs(tx.ID).WillReturnRows(rows)
		mock.ExpectQuery(tagsQuery).WithArgs(tx.ID).WillReturnRows(
			sqlmock.NewRows([]string{"tag_id"}).AddRow(tag),
		)

		got, err := s.FindTransactionByID(context.Background(), tx.ID)
		require.NoError(t, err)
		requireTransactionEqual(t, tx, got)
		require.Equal(t, []uint64{tag}, got.TagIDs)
	})

	t.Run("an unknown id is sql.ErrNoRows, no tag lookup issued", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(rowQuery).WithArgs(tx.ID).WillReturnError(sql.ErrNoRows)

		_, err := s.FindTransactionByID(context.Background(), tx.ID)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("a driver error on the tags lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		rows := sqlmock.NewRows(transactionRowColumns)
		addTransactionRow(rows, tx)
		mock.ExpectQuery(rowQuery).WithArgs(tx.ID).WillReturnRows(rows)
		mock.ExpectQuery(tagsQuery).WithArgs(tx.ID).WillReturnError(errStub)

		_, err := s.FindTransactionByID(context.Background(), tx.ID)
		require.ErrorIs(t, err, errStub)
	})
}

func fakeTransactionCreateRequest() model.TransactionCreateRequest {
	categoryID := fakeID()

	return model.TransactionCreateRequest{
		AccountID: fakeID(), CategoryID: &categoryID, Type: entity.TransactionTypeExpense,
		CurrencyCode: fakeCode(), Amount: fakeAmount(), Memo: fakeMemo(), OperationAt: fakeTime(),
	}
}

const (
	insertTransactionQuery = `INSERT INTO transactions (category_id, type, account_id, currency, amount, memo, operation_at)
			 VALUES (?, ?, ?, ?, ?, ?, ?)`
	deleteTagsQuery  = `DELETE FROM transaction_tags WHERE transaction_id = ?`
	insertTagQuery   = `INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)`
	txLookupQuery    = `SELECT account_id, amount FROM transactions WHERE id = ? FOR UPDATE`
	updateBalanceAdd = `UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	updateBalanceSub = `UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
)

// TestCreateTransactionWithBalance covers the insert-then-adjust-balance-then-tag transaction,
// including tagless and tagged creates, and every failure point rolling back.
func TestCreateTransactionWithBalance(t *testing.T) {
	t.Parallel()

	req := fakeTransactionCreateRequest()
	newID := int64(fakeID())

	t.Run("creates a tagless transaction and adjusts the balance", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertTransactionQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(uint64(newID)).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		got, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, uint64(newID), got.ID)
		require.Equal(t, req.Amount, got.Amount)
	})

	t.Run("creates a tagged transaction, one insert per tag", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		taggedReq := req
		taggedReq.TagIDs = []uint64{fakeID(), fakeID()}

		mock.ExpectBegin()
		mock.ExpectExec(insertTransactionQuery).
			WithArgs(
				taggedReq.CategoryID, uint8(taggedReq.Type), taggedReq.AccountID, taggedReq.CurrencyCode,
				taggedReq.Amount, taggedReq.Memo, taggedReq.OperationAt,
			).WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(taggedReq.Amount, taggedReq.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(uint64(newID)).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(insertTagQuery).WithArgs(uint64(newID), taggedReq.TagIDs[0]).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(insertTagQuery).WithArgs(uint64(newID), taggedReq.TagIDs[1]).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		got, err := s.CreateTransactionWithBalance(context.Background(), taggedReq)
		require.NoError(t, err)
		require.Equal(t, taggedReq.TagIDs, got.TagIDs)
	})

	t.Run("a driver error on the insert rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertTransactionQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance on the account update rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertTransactionQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(req.Amount, req.AccountID).WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("a driver error setting tags rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertTransactionQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(uint64(newID)).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})
}

func fakeTransactionUpdateRequest() model.TransactionUpdateRequest {
	categoryID := fakeID()

	return model.TransactionUpdateRequest{
		ID: fakeID(), AccountID: fakeID(), CategoryID: &categoryID, Type: entity.TransactionTypeExpense,
		CurrencyCode: fakeCode(), Amount: fakeAmount(), Memo: fakeMemo(), OperationAt: fakeTime(),
	}
}

const updateTransactionQuery = `UPDATE transactions
			 SET account_id = ?, category_id = ?, type = ?, currency = ?, amount = ?, memo = ?,
			     operation_at = ?, updated_at = CURRENT_TIMESTAMP
			 WHERE id = ?`

// TestUpdateTransactionWithBalance covers reversing the old balance effect and applying the new one
// — even across an account change — plus the not-found short circuit and every rollback path.
func TestUpdateTransactionWithBalance(t *testing.T) {
	t.Parallel()

	req := fakeTransactionUpdateRequest()
	oldAccountID, oldAmount := fakeID(), fakeAmount()

	t.Run("reverses the old effect and applies the new one", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(updateBalanceSub).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateTransactionQuery).
			WithArgs(
				req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID,
			).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(req.ID).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		got, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, req.ID, got.ID)
		require.Equal(t, req.Amount, got.Amount)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})

	t.Run("insufficient balance reversing the old effect rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(updateBalanceSub).WithArgs(oldAmount, oldAccountID).WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
		require.False(t, found)
	})

	t.Run("a driver error on the row update rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(updateBalanceSub).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateTransactionQuery).
			WithArgs(
				req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID,
			).WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})

	t.Run("insufficient balance applying the new effect rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(updateBalanceSub).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateTransactionQuery).
			WithArgs(
				req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID,
			).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(req.Amount, req.AccountID).WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
		require.False(t, found)
	})

	t.Run("a driver error setting tags rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(updateBalanceSub).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateTransactionQuery).
			WithArgs(
				req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID,
			).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateBalanceAdd).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(req.ID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})
}

// TestDeleteTransactionWithBalance covers removing a transaction and reversing its balance effect,
// plus the not-found short circuit and rollback paths.
func TestDeleteTransactionWithBalance(t *testing.T) {
	t.Parallel()

	id := fakeID()
	accountID, amount := fakeID(), fakeAmount()

	t.Run("deletes the row and reverses the balance", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateBalanceSub).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(id).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		found, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error deleting the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance reversing the effect rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(txLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateBalanceSub).WithArgs(amount, accountID).WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}
