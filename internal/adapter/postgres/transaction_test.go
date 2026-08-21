package postgres

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"raccounting/internal/domain/entity"
	storageError "raccounting/internal/storage/error"
	"raccounting/internal/storage/model"
)

const (
	transactionSelectColumns = `id, category_id, type, account_id, currency, amount, transfer_transaction_id, transfer_currency, transfer_amount, transfer_rate, transfer_account_id, memo, operation_at, created_at, updated_at`
	transactionQualifiedCols = `t.id, t.category_id, t.type, t.account_id, t.currency, t.amount, t.transfer_transaction_id, t.transfer_currency, t.transfer_amount, t.transfer_rate, t.transfer_account_id, t.memo, t.operation_at, t.created_at, t.updated_at`
)

func transactionRowColumns() []string {
	return []string{
		"id", "category_id", "type", "account_id", "currency", "amount",
		"transfer_transaction_id", "transfer_currency", "transfer_amount", "transfer_rate", "transfer_account_id",
		"memo", "operation_at", "created_at", "updated_at",
	}
}

// fakeTransactionRow returns a plain (non-transfer) expense row: CategoryID set, every Transfer*
// field nil.
func fakeTransactionRow() model.Transaction {
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

// transactionRowValues builds the driver.Value row scanTransaction expects, translating the model's
// nil pointers into SQL NULLs the way a real row would carry them.
func transactionRowValues(m model.Transaction) []driver.Value {
	var categoryID, transferTransactionID, transferCurrencyCode, transferAmount, transferRate, transferAccountID driver.Value

	if m.CategoryID != nil {
		categoryID = *m.CategoryID
	}

	if m.TransferTransactionID != nil {
		transferTransactionID = *m.TransferTransactionID
	}

	if m.TransferCurrencyCode != nil {
		transferCurrencyCode = *m.TransferCurrencyCode
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

	return []driver.Value{
		m.ID, categoryID, m.Type, m.AccountID, m.CurrencyCode, m.Amount,
		transferTransactionID, transferCurrencyCode, transferAmount, transferRate, transferAccountID,
		m.Memo, m.OperationAt, m.CreatedAt, m.UpdatedAt,
	}
}

// TestListTransactions covers the full listing and the N+1-avoiding tag attachment: every
// transaction's TagIDs come from one bulk transaction_tags query, keyed back onto each row by id.
func TestListTransactions(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + transactionSelectColumns + ` FROM transactions ORDER BY operation_at DESC, id DESC`
	tagsQuery := `SELECT transaction_id, tag_id FROM transaction_tags`

	t.Run("an empty result still queries tags, and yields no rows", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows(transactionRowColumns()))
		mock.ExpectQuery(tagsQuery).WillReturnRows(sqlmock.NewRows([]string{"transaction_id", "tag_id"}))

		got, err := s.ListTransactions(context.Background())
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("attaches each transaction's tags, or none", func(t *testing.T) {
		t.Parallel()

		tagged := fakeTransactionRow()
		untagged := fakeTransactionRow()
		tag1, tag2 := fakeID(), fakeID()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(
			sqlmock.NewRows(transactionRowColumns()).
				AddRow(transactionRowValues(tagged)...).
				AddRow(transactionRowValues(untagged)...),
		)
		mock.ExpectQuery(tagsQuery).WillReturnRows(
			sqlmock.NewRows([]string{"transaction_id", "tag_id"}).
				AddRow(tagged.ID, tag1).
				AddRow(tagged.ID, tag2),
		)

		got, err := s.ListTransactions(context.Background())
		require.NoError(t, err)
		require.Len(t, got, 2)
		require.Equal(t, []uint64{tag1, tag2}, got[0].TagIDs)
		require.Empty(t, got[1].TagIDs)
	})

	t.Run("a driver error on the primary query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnError(errStub)

		_, err := s.ListTransactions(context.Background())
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the tags query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WillReturnRows(sqlmock.NewRows(transactionRowColumns()))
		mock.ExpectQuery(tagsQuery).WillReturnError(errStub)

		_, err := s.ListTransactions(context.Background())
		require.ErrorIs(t, err, errStub)
	})
}

// TestFindTransactionByID covers the single-row lookup plus its per-transaction tag read.
func TestFindTransactionByID(t *testing.T) {
	t.Parallel()

	query := `SELECT ` + transactionSelectColumns + ` FROM transactions WHERE id = $1`
	tagsQuery := `SELECT tag_id FROM transaction_tags WHERE transaction_id = $1`
	row := fakeTransactionRow()
	tag1, tag2 := fakeID(), fakeID()

	t.Run("finds the row with its tags", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WithArgs(row.ID).WillReturnRows(
			sqlmock.NewRows(transactionRowColumns()).AddRow(transactionRowValues(row)...),
		)
		mock.ExpectQuery(tagsQuery).WithArgs(row.ID).WillReturnRows(
			sqlmock.NewRows([]string{"tag_id"}).AddRow(tag1).AddRow(tag2),
		)

		got, err := s.FindTransactionByID(context.Background(), row.ID)
		require.NoError(t, err)
		require.Equal(t, []uint64{tag1, tag2}, got.TagIDs)
	})

	t.Run("finds a row with no tags", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WithArgs(row.ID).WillReturnRows(
			sqlmock.NewRows(transactionRowColumns()).AddRow(transactionRowValues(row)...),
		)
		mock.ExpectQuery(tagsQuery).WithArgs(row.ID).WillReturnRows(sqlmock.NewRows([]string{"tag_id"}))

		got, err := s.FindTransactionByID(context.Background(), row.ID)
		require.NoError(t, err)
		require.Empty(t, got.TagIDs)
	})

	t.Run("an unknown id is sql.ErrNoRows, without querying tags", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WithArgs(row.ID).WillReturnError(sql.ErrNoRows)

		_, err := s.FindTransactionByID(context.Background(), row.ID)
		require.ErrorIs(t, err, sql.ErrNoRows)
	})

	t.Run("a driver error on the tags query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(query).WithArgs(row.ID).WillReturnRows(
			sqlmock.NewRows(transactionRowColumns()).AddRow(transactionRowValues(row)...),
		)
		mock.ExpectQuery(tagsQuery).WithArgs(row.ID).WillReturnError(errStub)

		_, err := s.FindTransactionByID(context.Background(), row.ID)
		require.ErrorIs(t, err, errStub)
	})
}

// TestTransactionUsage covers the two distinct-id reads used to gate "delete this account/category"
// UI.
func TestTransactionUsage(t *testing.T) {
	t.Parallel()

	accountQuery := `SELECT DISTINCT account_id FROM transactions`
	categoryQuery := `SELECT DISTINCT category_id FROM transactions WHERE category_id IS NOT NULL`

	t.Run("returns the referenced account and category ids", func(t *testing.T) {
		t.Parallel()

		accountID, categoryID := fakeID(), fakeID()

		s, mock := newMock(t)
		mock.ExpectQuery(accountQuery).WillReturnRows(sqlmock.NewRows([]string{"account_id"}).AddRow(accountID))
		mock.ExpectQuery(categoryQuery).WillReturnRows(sqlmock.NewRows([]string{"category_id"}).AddRow(categoryID))

		got, err := s.TransactionUsage(context.Background())
		require.NoError(t, err)
		require.Equal(t, []uint64{accountID}, got.AccountIDs)
		require.Equal(t, []uint64{categoryID}, got.CategoryIDs)
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

// TestBuildTransactionListWhere pins the WHERE clause and $N placeholder numbering
// ListTransactionsFiltered's count/sums/page queries all share, for each filter field alone and
// combined.
func TestBuildTransactionListWhere(t *testing.T) {
	t.Parallel()

	dateFrom := fakeTime()
	dateTo := fakeTime()
	accountID := fakeID()
	categoryID := fakeID()
	tagID := fakeID()
	txType := entity.TransactionTypeExpense
	search := "groceries"

	tests := []struct {
		name      string
		filter    model.TransactionListFilter
		wantWhere string
		wantArgs  []any
	}{
		{
			name:      "no filter fields yields no WHERE clause",
			filter:    model.TransactionListFilter{},
			wantWhere: "",
			wantArgs:  nil,
		},
		{
			name:      "DateFrom alone",
			filter:    model.TransactionListFilter{DateFrom: &dateFrom},
			wantWhere: " WHERE t.operation_at >= $1",
			wantArgs:  []any{dateFrom.Format(entity.DateLayout)},
		},
		{
			name:      "DateTo alone",
			filter:    model.TransactionListFilter{DateTo: &dateTo},
			wantWhere: " WHERE t.operation_at <= $1",
			wantArgs:  []any{dateTo.Format(entity.DateLayout)},
		},
		{
			name:      "AccountID alone",
			filter:    model.TransactionListFilter{AccountID: &accountID},
			wantWhere: " WHERE t.account_id = $1",
			wantArgs:  []any{accountID},
		},
		{
			name:      "CategoryID alone",
			filter:    model.TransactionListFilter{CategoryID: &categoryID},
			wantWhere: " WHERE t.category_id = $1",
			wantArgs:  []any{categoryID},
		},
		{
			name:      "Type alone",
			filter:    model.TransactionListFilter{Type: &txType},
			wantWhere: " WHERE t.type = $1",
			wantArgs:  []any{uint8(txType)},
		},
		{
			name:      "TagID alone",
			filter:    model.TransactionListFilter{TagID: &tagID},
			wantWhere: " WHERE EXISTS (SELECT 1 FROM transaction_tags tt WHERE tt.transaction_id = t.id AND tt.tag_id = $1)",
			wantArgs:  []any{tagID},
		},
		{
			name:      "Search alone",
			filter:    model.TransactionListFilter{Search: search},
			wantWhere: " WHERE (t.memo LIKE $1 OR c.name LIKE $2)",
			wantArgs:  []any{"%" + search + "%", "%" + search + "%"},
		},
		{
			name:      "AccountID and Search combined, in declaration order",
			filter:    model.TransactionListFilter{AccountID: &accountID, Search: search},
			wantWhere: " WHERE t.account_id = $1 AND (t.memo LIKE $2 OR c.name LIKE $3)",
			wantArgs:  []any{accountID, "%" + search + "%", "%" + search + "%"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			where, args := buildTransactionListWhere(tt.filter)
			require.Equal(t, tt.wantWhere, where)
			require.Equal(t, tt.wantArgs, args)
		})
	}
}

// TestListTransactionsFiltered covers pagination defaults, the Search-driven JOIN and its LIKE args,
// and error propagation from each of the count/sums/page sub-queries.
func TestListTransactionsFiltered(t *testing.T) {
	t.Parallel()

	t.Run("no filter applies pagination defaults and skips the tags query when empty", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(`SELECT `+transactionQualifiedCols+
			` FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT $1 OFFSET $2`).
			WithArgs(100, 0).
			WillReturnRows(sqlmock.NewRows(transactionRowColumns()))

		got, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.NoError(t, err)
		require.Equal(t, 0, got.TotalCount)
		require.Empty(t, got.Transactions)
		require.Empty(t, got.SumsByCurrency)
	})

	t.Run("a populated page attaches tags and reports per-currency sums", func(t *testing.T) {
		t.Parallel()

		row := fakeTransactionRow()
		tagID := fakeID()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}).AddRow(row.CurrencyCode, row.Amount))
		mock.ExpectQuery(`SELECT `+transactionQualifiedCols+
			` FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT $1 OFFSET $2`).
			WithArgs(100, 0).
			WillReturnRows(sqlmock.NewRows(transactionRowColumns()).AddRow(transactionRowValues(row)...))
		mock.ExpectQuery(`SELECT transaction_id, tag_id FROM transaction_tags WHERE transaction_id IN ($1)`).
			WithArgs(row.ID).
			WillReturnRows(sqlmock.NewRows([]string{"transaction_id", "tag_id"}).AddRow(row.ID, tagID))

		got, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.NoError(t, err)
		require.Equal(t, 1, got.TotalCount)
		require.Len(t, got.Transactions, 1)
		require.Equal(t, []uint64{tagID}, got.Transactions[0].TagIDs)
		require.Equal(t, map[string]int64{row.CurrencyCode: row.Amount}, got.SumsByCurrency)
	})

	t.Run("AccountID and Search join categories and bind the LIKE args after the filter args", func(t *testing.T) {
		t.Parallel()

		accountID := fakeID()
		filter := model.TransactionListFilter{AccountID: &accountID, Search: "rent"}
		like := "%rent%"

		from := `transactions t LEFT JOIN categories c ON c.id = t.category_id`
		where := ` WHERE t.account_id = $1 AND (t.memo LIKE $2 OR c.name LIKE $3)`

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM `+from+where).
			WithArgs(accountID, like, like).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM `+from+where+` GROUP BY t.currency`).
			WithArgs(accountID, like, like).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(`SELECT `+transactionQualifiedCols+` FROM `+from+where+
			` ORDER BY t.operation_at DESC, t.id DESC LIMIT $4 OFFSET $5`).
			WithArgs(accountID, like, like, 100, 0).
			WillReturnRows(sqlmock.NewRows(transactionRowColumns()))

		_, err := s.ListTransactionsFiltered(context.Background(), filter)
		require.NoError(t, err)
	})

	t.Run("custom pagination computes LIMIT/OFFSET from Page and PageSize", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(`SELECT `+transactionQualifiedCols+
			` FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT $1 OFFSET $2`).
			WithArgs(10, 20).
			WillReturnRows(sqlmock.NewRows(transactionRowColumns()))

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{Page: 3, PageSize: 10})
		require.NoError(t, err)
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
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the page query is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(`SELECT `+transactionQualifiedCols+
			` FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT $1 OFFSET $2`).
			WithArgs(100, 0).
			WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the tags-for-page query is propagated", func(t *testing.T) {
		t.Parallel()

		row := fakeTransactionRow()

		s, mock := newMock(t)
		mock.ExpectQuery(`SELECT COUNT(*) FROM transactions t`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
		mock.ExpectQuery(`SELECT t.currency, SUM(t.amount) FROM transactions t GROUP BY t.currency`).
			WillReturnRows(sqlmock.NewRows([]string{"currency", "sum"}))
		mock.ExpectQuery(`SELECT `+transactionQualifiedCols+
			` FROM transactions t ORDER BY t.operation_at DESC, t.id DESC LIMIT $1 OFFSET $2`).
			WithArgs(100, 0).
			WillReturnRows(sqlmock.NewRows(transactionRowColumns()).AddRow(transactionRowValues(row)...))
		mock.ExpectQuery(`SELECT transaction_id, tag_id FROM transaction_tags WHERE transaction_id IN ($1)`).
			WithArgs(row.ID).
			WillReturnError(errStub)

		_, err := s.ListTransactionsFiltered(context.Background(), model.TransactionListFilter{})
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
	}
}

// TestCreateTransactionWithBalance covers the three-write transaction behind creating a transaction:
// the row insert, the balance adjustment, and replacing its tag associations — all atomically.
func TestCreateTransactionWithBalance(t *testing.T) {
	t.Parallel()

	insertQuery := `INSERT INTO transactions (category_id, type, account_id, currency, amount, memo, operation_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`
	balanceQuery := `UPDATE accounts SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	deleteTagsQuery := `DELETE FROM transaction_tags WHERE transaction_id = $1`
	insertTagQuery := `INSERT INTO transaction_tags (transaction_id, tag_id) VALUES ($1, $2)`

	req := fakeTransactionCreateRequest()
	wantID := fakeID()

	t.Run("inserts the row, adjusts the balance, and sets no tags", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
		mock.ExpectExec(balanceQuery).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(wantID).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		got, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, wantID, got.ID)
		require.Equal(t, req.Amount, got.Amount)
	})

	t.Run("replaces tags when TagIDs is non-empty", func(t *testing.T) {
		t.Parallel()

		tagged := req
		tagged.TagIDs = []uint64{fakeID(), fakeID()}

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertQuery).
			WithArgs(tagged.CategoryID, uint8(tagged.Type), tagged.AccountID, tagged.CurrencyCode, tagged.Amount, tagged.Memo, tagged.OperationAt).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
		mock.ExpectExec(balanceQuery).WithArgs(tagged.Amount, tagged.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(wantID).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectExec(insertTagQuery).WithArgs(wantID, tagged.TagIDs[0]).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(insertTagQuery).WithArgs(wantID, tagged.TagIDs[1]).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		got, err := s.CreateTransactionWithBalance(context.Background(), tagged)
		require.NoError(t, err)
		require.Equal(t, tagged.TagIDs, got.TagIDs)
	})

	t.Run("a failure inserting the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
		mock.ExpectExec(balanceQuery).WithArgs(req.Amount, req.AccountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("a failure replacing tags rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertQuery).
			WithArgs(req.CategoryID, uint8(req.Type), req.AccountID, req.CurrencyCode, req.Amount, req.Memo, req.OperationAt).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(wantID))
		mock.ExpectExec(balanceQuery).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(wantID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
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
	}
}

// TestUpdateTransactionWithBalance covers reversing the old balance effect, applying the new one —
// even across an account change — updating the row, and replacing tags, all atomically. The initial
// lookup uses SELECT ... FOR UPDATE, unlike the sqlite adapter (whose single-connection pool doesn't
// need row locking).
func TestUpdateTransactionWithBalance(t *testing.T) {
	t.Parallel()

	lookupQuery := `SELECT account_id, amount FROM transactions WHERE id = $1 FOR UPDATE`
	reverseBalanceQuery := `UPDATE accounts SET balance = balance - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	updateQuery := `UPDATE transactions
		 SET account_id = $1, category_id = $2, type = $3, currency = $4, amount = $5, memo = $6,
		     operation_at = $7, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $8`
	applyBalanceQuery := `UPDATE accounts SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`
	deleteTagsQuery := `DELETE FROM transaction_tags WHERE transaction_id = $1`

	req := fakeTransactionUpdateRequest()
	oldAccountID := fakeID()
	oldAmount := -fakeAmount()

	t.Run("reverses the old effect, applies the new one, and updates the row", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(reverseBalanceQuery).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo, req.OperationAt, req.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(applyBalanceQuery).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(req.ID).WillReturnResult(sqlmock.NewResult(0, 0))
		mock.ExpectCommit()

		got, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, req.ID, got.ID)
		require.Equal(t, req.AccountID, got.AccountID)
	})

	t.Run("an unknown id is reported as not found, without a side effect", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, _, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance reversing the old effect rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(reverseBalanceQuery).WithArgs(oldAmount, oldAccountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
		require.False(t, found)
	})

	t.Run("a driver error updating the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(reverseBalanceQuery).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo, req.OperationAt, req.ID).
			WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})

	t.Run("insufficient balance applying the new effect rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(reverseBalanceQuery).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo, req.OperationAt, req.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(applyBalanceQuery).WithArgs(req.Amount, req.AccountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
		require.False(t, found)
	})

	t.Run("a failure replacing tags rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(req.ID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(oldAccountID, oldAmount),
		)
		mock.ExpectExec(reverseBalanceQuery).WithArgs(oldAmount, oldAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(updateQuery).
			WithArgs(req.AccountID, req.CategoryID, uint8(req.Type), req.CurrencyCode, req.Amount, req.Memo, req.OperationAt, req.ID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(applyBalanceQuery).WithArgs(req.Amount, req.AccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTagsQuery).WithArgs(req.ID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, found, err := s.UpdateTransactionWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
		require.False(t, found)
	})
}

// TestDeleteTransactionWithBalance covers removing the row and reversing its balance effect,
// atomically.
func TestDeleteTransactionWithBalance(t *testing.T) {
	t.Parallel()

	lookupQuery := `SELECT account_id, amount FROM transactions WHERE id = $1 FOR UPDATE`
	deleteQuery := `DELETE FROM transactions WHERE id = $1`
	balanceQuery := `UPDATE accounts SET balance = balance - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`

	id := fakeID()
	accountID := fakeID()
	amount := -fakeAmount()

	t.Run("deletes the row and reverses the balance", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		found, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error deleting the row rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance reversing the effect rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(accountID, amount),
		)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(amount, accountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, err := s.DeleteTransactionWithBalance(context.Background(), id)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}
