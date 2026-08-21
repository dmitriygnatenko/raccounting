package postgres

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

func fakeTransferCreateRequest() model.TransferCreateRequest {
	return model.TransferCreateRequest{
		FromAccountID:    fakeID(),
		FromCurrencyCode: fakeCurrencyCode(),
		ToAccountID:      fakeID(),
		ToCurrencyCode:   fakeCurrencyCode(),
		Amount:           fakeAmount(),
		CreditAmount:     fakeAmount(),
		Rate:             fakeRate(),
		OperationAt:      fakeTime(),
		Memo:             fakeMemo(),
	}
}

// TestCreateTransferWithBalance covers the four-write transaction behind creating a transfer: both
// legs inserted (pointing at each other via transfer_transaction_id), the link back-filled onto the
// first leg, and both accounts' balances adjusted — all atomically. Every failure before Commit has
// to roll back rather than leave a half-applied transfer.
func TestCreateTransferWithBalance(t *testing.T) {
	t.Parallel()

	insertFromQuery := `INSERT INTO transactions
		 (category_id, type, account_id, currency, amount, transfer_currency, transfer_amount,
		  transfer_rate, transfer_account_id, memo, operation_at)
		 VALUES (NULL, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`
	insertToQuery := `INSERT INTO transactions
		 (category_id, type, account_id, currency, amount, transfer_transaction_id, transfer_currency,
		  transfer_amount, transfer_rate, transfer_account_id, memo, operation_at)
		 VALUES (NULL, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11) RETURNING id`
	linkQuery := `UPDATE transactions SET transfer_transaction_id = $1 WHERE id = $2`
	balanceQuery := `UPDATE accounts SET balance = balance + $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`

	req := fakeTransferCreateRequest()
	debitAmount := -req.Amount
	fromID, toID := fakeID(), fakeID()

	t.Run("creates both legs and adjusts both balances, atomically", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertFromQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fromID))
		mock.ExpectQuery(insertToQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(toID))
		mock.ExpectExec(linkQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(debitAmount, req.FromAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(req.CreditAmount, req.ToAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		got, err := s.CreateTransferWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, fromID, got.LegFrom.ID)
		require.NotNil(t, got.LegFrom.TransferTransactionID)
		require.Equal(t, toID, *got.LegFrom.TransferTransactionID)
		require.Equal(t, toID, got.LegTo.ID)
		require.NotNil(t, got.LegTo.TransferTransactionID)
		require.Equal(t, fromID, *got.LegTo.TransferTransactionID)
		require.Equal(t, debitAmount, got.LegFrom.Amount)
		require.Equal(t, req.CreditAmount, got.LegTo.Amount)
	})

	t.Run("a failure inserting the first leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertFromQuery).WithArgs(
			uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
			req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
		).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a failure inserting the second leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertFromQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fromID))
		mock.ExpectQuery(insertToQuery).WithArgs(
			uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
			fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
		).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a failure linking the legs rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertFromQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fromID))
		mock.ExpectQuery(insertToQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(toID))
		mock.ExpectExec(linkQuery).WithArgs(toID, fromID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance on the debit leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertFromQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fromID))
		mock.ExpectQuery(insertToQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(toID))
		mock.ExpectExec(linkQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(debitAmount, req.FromAccountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("insufficient balance on the credit leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(insertFromQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(fromID))
		mock.ExpectQuery(insertToQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(toID))
		mock.ExpectExec(linkQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(debitAmount, req.FromAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(req.CreditAmount, req.ToAccountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}

// TestDeleteTransferWithBalance covers reversing both legs of a transfer, atomically, including the
// single-leg case (the paired leg was already deleted, or never linked) and the not-found short
// circuit.
func TestDeleteTransferWithBalance(t *testing.T) {
	t.Parallel()

	lookupQuery := `SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = $1 FOR UPDATE`
	otherLookupQuery := `SELECT account_id, amount FROM transactions WHERE id = $1 FOR UPDATE`
	deleteQuery := `DELETE FROM transactions WHERE id = $1`
	balanceQuery := `UPDATE accounts SET balance = balance - $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2`

	id := fakeID()
	accountID := fakeID()
	amount := fakeAmount()

	t.Run("an unknown id is reported as not found, without a transaction side effect", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a leg with no paired transaction reverses only itself", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, nil),
		)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	otherID := fakeID()
	otherAccountID := fakeID()
	otherAmount := fakeAmount()

	t.Run("both legs are deleted and both balances reversed, atomically", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, otherID),
		)
		mock.ExpectQuery(otherLookupQuery).WithArgs(otherID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(otherAccountID, otherAmount),
		)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteQuery).WithArgs(otherID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(otherAmount, otherAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("the paired leg having vanished already is not an error, only this leg is reversed", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, otherID),
		)
		mock.ExpectQuery(otherLookupQuery).WithArgs(otherID).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("insufficient balance reversing a leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(lookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, nil),
		)
		mock.ExpectExec(deleteQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(balanceQuery).WithArgs(amount, accountID).WillReturnError(balanceCheckErr())
		mock.ExpectRollback()

		_, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}
