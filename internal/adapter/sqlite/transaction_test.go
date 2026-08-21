package sqlite

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

var transactionColumnNames = []string{
	"id", "category_id", "type", "account_id", "currency", "amount",
	"transfer_transaction_id", "transfer_currency", "transfer_amount", "transfer_rate", "transfer_account_id",
	"memo", "operation_at", "created_at", "updated_at",
}

// fakeTransaction returns a random plain (non-transfer) model.Transaction, as scanTransaction would
// produce it before TagIDs is hydrated by a separate query.
func fakeTransaction() model.Transaction {
	categoryID := fakeID()

	return model.Transaction{
		ID:           fakeID(),
		CategoryID:   &categoryID,
		Type:         uint8(entity.TransactionTypeExpense),
		AccountID:    fakeID(),
		CurrencyCode: fakeCurrencyCode(),
		Amount:       -fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeTime(),
		CreatedAt:    fakeTime(),
		UpdatedAt:    fakeTime(),
	}
}

// addTransactionRow appends m to rows in transactionColumns order, translating the model's nil
// pointer fields into SQL NULLs the way a real transfer-less row would come back.
func addTransactionRow(rows *sqlmock.Rows, m model.Transaction) *sqlmock.Rows {
	var categoryID, transferTxID, transferCurrency, transferAmount, transferRate, transferAccountID any

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

const transactionListQuery = `SELECT id, category_id, type, account_id, currency, amount,
	transfer_transaction_id, transfer_currency, transfer_amount, transfer_rate, transfer_account_id,
	memo, operation_at, created_at, updated_at FROM transactions ORDER BY operation_at DESC, id DESC`

const transactionTagsAllQuery = `SELECT transaction_id, tag_id FROM transaction_tags`

// TestListTransactions covers the full listing plus its N+1-avoiding tag hydration: every row comes
// back scanned into model.Transaction, and each one's TagIDs are filled in from a single follow-up
// query rather than one query per row.
func TestListTransactions(t *testing.T) {
	t.Parallel()

	t1, t2 := fakeTransaction(), fakeTransaction()
	tag1, tag2 := fakeID(), fakeID()

	t.Run("an empty result yields no rows and skips nothing", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(transactionListQuery).WillReturnRows(sqlmock.NewRows(transactionColumnNames))
		mock.ExpectQuery(transactionTagsAllQuery).WillReturnRows(sqlmock.NewRows([]string{"transaction_id", "tag_id"}))

		got, err := s.ListTransactions(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("hydrates each row's tags from the single follow-up query", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mockRows := sqlmock.NewRows(transactionColumnNames)
		addTransactionRow(mockRows, t1)
		addTransactionRow(mockRows, t2)
		mock.ExpectQuery(transactionListQuery).WillReturnRows(mockRows)

		mock.ExpectQuery(transactionTagsAllQuery).WillReturnRows(
			sqlmock.NewRows([]string{"transaction_id", "tag_id"}).
				AddRow(t1.ID, tag1).
				AddRow(t1.ID, tag2).
				AddRow(t2.ID, tag1),
		)

		got, err := s.ListTransactions(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 2)

		want1, want2 := t1, t2
		want1.TagIDs = []uint64{tag1, tag2}
		want2.TagIDs = []uint64{tag1}
		require.Equal(t, want1, got[0])
		require.Equal(t, want2, got[1])
	})

	t.Run("a driver error on the listing query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(transactionListQuery).WillReturnError(errStub)

		_, err := s.ListTransactions(context.Background())
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the tags query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(transactionListQuery).WillReturnRows(
			addTransactionRow(sqlmock.NewRows(transactionColumnNames), t1),
		)
		mock.ExpectQuery(transactionTagsAllQuery).WillReturnError(errStub)

		_, err := s.ListTransactions(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestBuildTransactionListWhere covers every filter field alone and combined — the SQL fragment and
// bound args every ListTransactionsFiltered sub-query shares.
func TestBuildTransactionListWhere(t *testing.T) {
	t.Parallel()

	dateFrom := fakeTime()
	dateTo := fakeTime()
	accountID := fakeID()
	categoryID := fakeID()
	tagID := fakeID()
	txType := entity.TransactionTypeExpense

	tests := []struct {
		name      string
		filter    model.TransactionListFilter
		wantWhere string
		wantArgs  []any
	}{
		{
			name:      "no filter produces no clause",
			filter:    model.TransactionListFilter{},
			wantWhere: "",
			wantArgs:  nil,
		},
		{
			name:      "DateFrom alone",
			filter:    model.TransactionListFilter{DateFrom: &dateFrom},
			wantWhere: " WHERE t.operation_at >= ?",
			wantArgs:  []any{dateFrom.Format(entity.DateLayout)},
		},
		{
			name:      "DateTo alone",
			filter:    model.TransactionListFilter{DateTo: &dateTo},
			wantWhere: " WHERE t.operation_at <= ?",
			wantArgs:  []any{dateTo.Format(entity.DateLayout)},
		},
		{
			name:      "AccountID alone",
			filter:    model.TransactionListFilter{AccountID: &accountID},
			wantWhere: " WHERE t.account_id = ?",
			wantArgs:  []any{accountID},
		},
		{
			name:      "CategoryID alone",
			filter:    model.TransactionListFilter{CategoryID: &categoryID},
			wantWhere: " WHERE t.category_id = ?",
			wantArgs:  []any{categoryID},
		},
		{
			name:      "Type alone",
			filter:    model.TransactionListFilter{Type: &txType},
			wantWhere: " WHERE t.type = ?",
			wantArgs:  []any{uint8(txType)},
		},
		{
			name:      "TagID alone",
			filter:    model.TransactionListFilter{TagID: &tagID},
			wantWhere: " WHERE EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id AND tt.tag_id = ?)",
			wantArgs:  []any{tagID},
		},
		{
			name:      "Search alone",
			filter:    model.TransactionListFilter{Search: "coffee"},
			wantWhere: " WHERE (t.memo LIKE ? OR c.name LIKE ?)",
			wantArgs:  []any{"%coffee%", "%coffee%"},
		},
		{
			name: "every field combined, joined with AND in declaration order",
			filter: model.TransactionListFilter{
				DateFrom: &dateFrom, DateTo: &dateTo, AccountID: &accountID, CategoryID: &categoryID,
				Type: &txType, TagID: &tagID, Search: "coffee",
			},
			wantWhere: " WHERE t.operation_at >= ? AND t.operation_at <= ? AND t.account_id = ? AND t.category_id = ?" +
				" AND t.type = ? AND EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id AND tt.tag_id = ?)" +
				" AND (t.memo LIKE ? OR c.name LIKE ?)",
			wantArgs: []any{
				dateFrom.Format(entity.DateLayout), dateTo.Format(entity.DateLayout), accountID, categoryID,
				uint8(txType), tagID, "%coffee%", "%coffee%",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotWhere, gotArgs := buildTransactionListWhere(tt.filter)
			require.Equal(t, tt.wantWhere, gotWhere)
			require.Equal(t, tt.wantArgs, gotArgs)
		})
	}
}

// TestListTransactionsFiltered covers the no-filter path, a filter combining AccountID and Search
// (which pulls in the categories join), pagination defaults, and error propagation from each of the
// count/sums/page/tags sub-queries.
func TestListTransactionsFiltered(t *testing.T) {
	t.Parallel()

	t.Run("no filter defaults to page 1, page size 100", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).WillReturnRows(
			sqlmock.NewRows([]string{"count"}).AddRow(0),
		)
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(
			`SELECT t.id, t.category_id, t.type, t.account_id, t.currency, t.amount,
	t.transfer_transaction_id, t.transfer_currency, t.transfer_amount, t.transfer_rate, t.transfer_account_id,
	t.memo, t.operation_at, t.created_at, t.updated_at FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`,
		).WithArgs(100, 0).WillReturnRows(sqlmock.NewRows(transactionColumnNames))

		got, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.NoError(t, err)
		require.Equal(t, 0, got.TotalCount)
		require.Empty(t, got.Transactions)
		require.Empty(t, got.SumsByCurrency)
	})

	t.Run("AccountID + Search pulls in the categories join and pages the result", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		accountID := fakeID()
		filter := model.TransactionListFilter{AccountID: &accountID, Search: "coffee", Page: 2, PageSize: 10}

		txn := fakeTransaction()
		tag := fakeID()

		countQuery := `SELECT COUNT(*) FROM transactions t LEFT JOIN categories c ON c.id = t.category_id` +
			` WHERE t.account_id = ? AND (t.memo LIKE ? OR c.name LIKE ?)`
		sumsQuery := `SELECT t.currency, SUM(t.amount) FROM transactions t LEFT JOIN categories c ON c.id = t.category_id` +
			` WHERE t.account_id = ? AND (t.memo LIKE ? OR c.name LIKE ?) GROUP BY t.currency`
		pageQuery := `SELECT t.id, t.category_id, t.type, t.account_id, t.currency, t.amount,
	t.transfer_transaction_id, t.transfer_currency, t.transfer_amount, t.transfer_rate, t.transfer_account_id,
	t.memo, t.operation_at, t.created_at, t.updated_at FROM transactions t LEFT JOIN categories c ON c.id = t.category_id` +
			` WHERE t.account_id = ? AND (t.memo LIKE ? OR c.name LIKE ?) ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`

		mock.ExpectQuery(countQuery).WithArgs(accountID, "%coffee%", "%coffee%").
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(sumsQuery).WithArgs(accountID, "%coffee%", "%coffee%").
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}).AddRow(txn.CurrencyCode, txn.Amount))
		mock.ExpectQuery(pageQuery).WithArgs(accountID, "%coffee%", "%coffee%", 10, 10).
			WillReturnRows(addTransactionRow(sqlmock.NewRows(transactionColumnNames), txn))
		mock.ExpectQuery(`SELECT transaction_id, tag_id FROM transaction_tags WHERE transaction_id IN (?)`).
			WithArgs(txn.ID).
			WillReturnRows(sqlmock.NewRows([]string{"transaction_id", "tag_id"}).AddRow(txn.ID, tag))

		got, err := s.ListTransactionsFiltered(context.Background(), filter)
		require.NoError(t, err)
		require.Equal(t, 1, got.TotalCount)
		require.Equal(t, map[string]int64{txn.CurrencyCode: txn.Amount}, got.SumsByCurrency)
		require.Len(t, got.Transactions, 1)

		want := txn
		want.TagIDs = []uint64{tag}
		require.Equal(t, want, got.Transactions[0])
	})

	t.Run("a driver error on the count query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the sums query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the page query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(
			`SELECT t.id, t.category_id, t.type, t.account_id, t.currency, t.amount,
	t.transfer_transaction_id, t.transfer_currency, t.transfer_amount, t.transfer_rate, t.transfer_account_id,
	t.memo, t.operation_at, t.created_at, t.updated_at FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`,
		).WithArgs(100, 0).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error hydrating tags for the page is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		txn := fakeTransaction()

		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(
			`SELECT t.id, t.category_id, t.type, t.account_id, t.currency, t.amount,
	t.transfer_transaction_id, t.transfer_currency, t.transfer_amount, t.transfer_rate, t.transfer_account_id,
	t.memo, t.operation_at, t.created_at, t.updated_at FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT ? OFFSET ?`,
		).WithArgs(100, 0).WillReturnRows(addTransactionRow(sqlmock.NewRows(transactionColumnNames), txn))
		mock.ExpectQuery(`SELECT transaction_id, tag_id FROM transaction_tags WHERE transaction_id IN (?)`).
			WithArgs(txn.ID).WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})
}

// TestTransactionUsage covers the two distinct-id sweeps the "can this account/category be deleted?"
// UI gate is built from.
func TestTransactionUsage(t *testing.T) {
	t.Parallel()

	accountID1, accountID2 := fakeID(), fakeID()
	categoryID := fakeID()

	t.Run("returns the distinct account and category ids in use", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectQuery(`SELECT DISTINCT account_id FROM transactions`).WillReturnRows(
			sqlmock.NewRows([]string{"account_id"}).AddRow(accountID1).AddRow(accountID2),
		)
		mock.ExpectQuery(`SELECT DISTINCT category_id FROM transactions WHERE category_id IS NOT NULL`).WillReturnRows(
			sqlmock.NewRows([]string{"category_id"}).AddRow(categoryID),
		)

		got, err := s.TransactionUsage(context.Background())
		require.NoError(t, err)
		require.Equal(t, model.TransactionUsage{
			AccountIDs:  []uint64{accountID1, accountID2},
			CategoryIDs: []uint64{categoryID},
		}, got)
	})

	t.Run("a driver error on the account sweep is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT DISTINCT account_id FROM transactions`).WillReturnError(errStub)

		_, err := s.TransactionUsage(context.Background())
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the category sweep is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT DISTINCT account_id FROM transactions`).WillReturnRows(
			sqlmock.NewRows([]string{"account_id"}).AddRow(accountID1),
		)
		mock.ExpectQuery(`SELECT DISTINCT category_id FROM transactions WHERE category_id IS NOT NULL`).
			WillReturnError(errStub)

		_, err := s.TransactionUsage(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindTransactionByID covers the single-row lookup plus its tag hydration, and the not-found
// case, where the tag lookup never happens.
func TestFindTransactionByID(t *testing.T) {
	t.Parallel()

	txn := fakeTransaction()
	tag := fakeID()

	t.Run("finds the row and hydrates its tags", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectQuery(`SELECT id, category_id, type, account_id, currency, amount,
	transfer_transaction_id, transfer_currency, transfer_amount, transfer_rate, transfer_account_id,
	memo, operation_at, created_at, updated_at FROM transactions WHERE id = ?`).WithArgs(txn.ID).WillReturnRows(
			addTransactionRow(sqlmock.NewRows(transactionColumnNames), txn),
		)
		mock.ExpectQuery(`SELECT tag_id FROM transaction_tags WHERE transaction_id = ?`).WithArgs(txn.ID).
			WillReturnRows(sqlmock.NewRows([]string{"tag_id"}).AddRow(tag))

		got, err := s.FindTransactionByID(context.Background(), txn.ID)
		require.NoError(t, err)

		want := txn
		want.TagIDs = []uint64{tag}
		require.Equal(t, want, got)
	})

	t.Run("an unknown id is sql.ErrNoRows, no tag lookup", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT id, category_id, type, account_id, currency, amount,
	transfer_transaction_id, transfer_currency, transfer_amount, transfer_rate, transfer_account_id,
	memo, operation_at, created_at, updated_at FROM transactions WHERE id = ?`).WithArgs(txn.ID).
			WillReturnError(sql.ErrNoRows)

		_, err := s.FindTransactionByID(context.Background(), txn.ID)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("a driver error hydrating tags is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT id, category_id, type, account_id, currency, amount,
	transfer_transaction_id, transfer_currency, transfer_amount, transfer_rate, transfer_account_id,
	memo, operation_at, created_at, updated_at FROM transactions WHERE id = ?`).WithArgs(txn.ID).WillReturnRows(
			addTransactionRow(sqlmock.NewRows(transactionColumnNames), txn),
		)
		mock.ExpectQuery(`SELECT tag_id FROM transaction_tags WHERE transaction_id = ?`).WithArgs(txn.ID).
			WillReturnError(errStub)

		_, err := s.FindTransactionByID(context.Background(), txn.ID)
		require.ErrorIs(t, err, errStub)
	})
}

func fakeTransactionCreateRequest() model.TransactionCreateRequest {
	categoryID := fakeID()

	return model.TransactionCreateRequest{
		AccountID:    fakeID(),
		CategoryID:   &categoryID,
		Type:         entity.TransactionTypeExpense,
		CurrencyCode: fakeCurrencyCode(),
		Amount:       -fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeTime(),
		TagIDs:       []uint64{fakeID(), fakeID()},
	}
}

const transactionInsertQuery = `INSERT INTO transactions (category_id, type, account_id, currency, amount, memo, operation_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`

const transactionBalanceUpdateQuery = `UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

const transactionTagsDeleteQuery = `DELETE FROM transaction_tags WHERE transaction_id = ?`

const transactionTagInsertQuery = `INSERT INTO transaction_tags (transaction_id, tag_id) VALUES (?, ?)`

// TestCreateTransactionWithBalance covers the happy path — insert, balance adjustment, and tag
// replacement, all inside one transaction — plus each step's failure mode rolling back.
func TestCreateTransactionWithBalance(t *testing.T) {
	t.Parallel()

	req := fakeTransactionCreateRequest()
	newID := int64(fakeID())

	t.Run("inserts, adjusts the balance, sets tags, and commits", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transactionInsertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(transactionBalanceUpdateQuery).WithArgs(req.Amount, req.AccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionTagsDeleteQuery).WithArgs(uint64(newID)).WillReturnResult(sqlmock.NewResult(0, 0))
		for _, tagID := range req.TagIDs {
			mock.ExpectExec(transactionTagInsertQuery).WithArgs(uint64(newID), tagID).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		mock.ExpectCommit()

		got, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, uint64(newID), got.ID)
		require.Equal(t, req.AccountID, got.AccountID)
		require.Equal(t, req.Amount, got.Amount)
		require.Equal(t, req.TagIDs, got.TagIDs)
	})

	t.Run("a failure inserting the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transactionInsertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("overdrawing the account rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transactionInsertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(transactionBalanceUpdateQuery).WithArgs(req.Amount, req.AccountID).
			WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("a failure setting tags rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transactionInsertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(transactionBalanceUpdateQuery).WithArgs(req.Amount, req.AccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionTagsDeleteQuery).WithArgs(uint64(newID)).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("no tags means the delete runs but no inserts follow", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		noTagsReq := req
		noTagsReq.TagIDs = nil

		mock.ExpectBegin()
		mock.ExpectExec(transactionInsertQuery).
			WithArgs(noTagsReq.CategoryID, uint8(noTagsReq.Type), noTagsReq.AccountID, noTagsReq.CurrencyCode,
				noTagsReq.Amount, noTagsReq.Memo, noTagsReq.OperationAt).
			WillReturnResult(sqlmock.NewResult(newID, 1))
		mock.ExpectExec(transactionBalanceUpdateQuery).WithArgs(noTagsReq.Amount, noTagsReq.AccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionTagsDeleteQuery).WithArgs(uint64(newID)).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		got, err := s.CreateTransactionWithBalance(context.Background(), noTagsReq)
		require.NoError(t, err)
		require.Empty(t, got.TagIDs)
	})
}

func fakeTransactionUpdateRequest() model.TransactionUpdateRequest {
	categoryID := fakeID()

	return model.TransactionUpdateRequest{
		ID:           fakeID(),
		AccountID:    fakeID(),
		CategoryID:   &categoryID,
		Type:         entity.TransactionTypeExpense,
		CurrencyCode: fakeCurrencyCode(),
		Amount:       -fakeAmount(),
		Memo:         fakeMemo(),
		OperationAt:  fakeTime(),
		TagIDs:       []uint64{fakeID()},
	}
}

const transactionOldLookupQuery = `SELECT account_id, amount FROM transactions WHERE id = ?`

const transactionBalanceReverseQuery = `UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`

const transactionUpdateQuery = `UPDATE transactions
		 SET account_id = ?, category_id = ?, type = ?, currency = ?, amount = ?, memo = ?,
		     operation_at = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ?`

// TestUpdateTransactionWithBalance covers reversing the old balance effect, applying the new one
// (even across an account change), replacing tags, and committing — plus the not-found case and each
// step's failure mode rolling back.
func TestUpdateTransactionWithBalance(t *testing.T) {
	t.Parallel()

	req := fakeTransactionUpdateRequest()
	oldAccountID := fakeID()
	oldAmount := -fakeAmount()

	t.Run("reverses the old effect, applies the new one, replaces tags, and commits", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(transactionBalanceReverseQuery).WithArgs(oldAmount, oldAccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionUpdateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionBalanceUpdateQuery).WithArgs(req.Amount, req.AccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionTagsDeleteQuery).WithArgs(req.ID).WillReturnResult(sqlmock.NewResult(0, 0))
		for _, tagID := range req.TagIDs {
			mock.ExpectExec(transactionTagInsertQuery).WithArgs(req.ID, tagID).WillReturnResult(sqlmock.NewResult(0, 1))
		}
		mock.ExpectCommit()

		got, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, req.ID, got.ID)
		require.Equal(t, req.AccountID, got.AccountID)
		require.Equal(t, req.Amount, got.Amount)
		require.Equal(t, req.TagIDs, got.TagIDs)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(req.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the initial lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(req.ID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})

	t.Run("reversing the old effect below zero rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(transactionBalanceReverseQuery).WithArgs(oldAmount, oldAccountID).
			WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
		require.False(t, found)
	})

	t.Run("applying the new effect over the limit rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(transactionBalanceReverseQuery).WithArgs(oldAmount, oldAccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionUpdateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionBalanceUpdateQuery).WithArgs(req.Amount, req.AccountID).
			WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
		require.False(t, found)
	})

	t.Run("a failure updating the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(transactionBalanceReverseQuery).WithArgs(oldAmount, oldAccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionUpdateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo,
				req.OperationAt, req.ID).
			WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})
}

// TestDeleteTransactionWithBalance covers removing the row and reversing its balance effect, the
// not-found case, and each step's failure mode rolling back.
func TestDeleteTransactionWithBalance(t *testing.T) {
	t.Parallel()

	id := fakeID()
	accountID := fakeID()
	amount := -fakeAmount()

	t.Run("deletes the row and reverses its balance effect", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionBalanceReverseQuery).WithArgs(amount, accountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(id).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		found, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a failure deleting the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("reversing the balance below zero rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(transactionOldLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transactionBalanceReverseQuery).WithArgs(amount, accountID).
			WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}
