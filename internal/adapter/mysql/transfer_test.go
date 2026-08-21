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

const (
	insertFromLegQuery = `INSERT INTO transactions
			 (category_id, type, account_id, currency, amount, transfer_currency, transfer_amount,
			  transfer_rate, transfer_account_id, memo, operation_at)
			 VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	insertToLegQuery = `INSERT INTO transactions
			 (category_id, type, account_id, currency, amount, transfer_transaction_id, transfer_currency,
			  transfer_amount, transfer_rate, transfer_account_id, memo, operation_at)
			 VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	linkLegsQuery       = `UPDATE transactions SET transfer_transaction_id = ? WHERE id = ?`
	adjustBalanceQuery  = `UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
	transferLookupQuery = `SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ? FOR UPDATE`
	otherLegLookupQuery = `SELECT account_id, amount FROM transactions WHERE id = ? FOR UPDATE`
	deleteTxQuery       = `DELETE FROM transactions WHERE id = ?`
	reverseBalanceQuery = `UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
)

func fakeTransferCreateRequest() model.TransferCreateRequest {
	amount, creditAmount := fakeAmount(), fakeAmount()

	return model.TransferCreateRequest{
		FromAccountID: fakeID(), FromCurrencyCode: fakeCode(),
		ToAccountID: fakeID(), ToCurrencyCode: fakeCode(),
		Amount: amount, CreditAmount: creditAmount, Rate: fakeRate(),
		OperationAt: fakeTime(), Memo: fakeMemo(),
	}
}

// TestCreateTransferWithBalance covers the six-statement transaction behind creating a transfer:
// both legs inserted, linked to each other, and both accounts' balances adjusted — all atomically.
func TestCreateTransferWithBalance(t *testing.T) {
	t.Parallel()

	req := fakeTransferCreateRequest()
	debitAmount := -req.Amount
	fromID, toID := int64(fakeID()), int64(fakeID())

	t.Run("creates both legs and adjusts both balances", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertFromLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(insertToLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(linkLegsQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(adjustBalanceQuery).WithArgs(debitAmount, req.FromAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(adjustBalanceQuery).WithArgs(req.CreditAmount, req.ToAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		got, err := s.CreateTransferWithBalance(context.Background(), req)
		require.NoError(t, err)
		require.Equal(t, uint64(fromID), got.LegFrom.ID)
		require.Equal(t, uint64(toID), got.LegTo.ID)
		require.Equal(t, debitAmount, got.LegFrom.Amount)
		require.Equal(t, req.CreditAmount, got.LegTo.Amount)
		require.Equal(t, uint64(toID), *got.LegFrom.TransferTransactionID)
		require.Equal(t, uint64(fromID), *got.LegTo.TransferTransactionID)
	})

	t.Run("a driver error on the debit leg insert rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertFromLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error on the credit leg insert rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertFromLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(insertToLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a driver error linking the legs rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertFromLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(insertToLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(linkLegsQuery).WithArgs(toID, fromID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("insufficient balance on the debit leg's account rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertFromLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(insertToLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(linkLegsQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(adjustBalanceQuery).WithArgs(debitAmount, req.FromAccountID).
			WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("insufficient balance on the credit leg's account rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectExec(insertFromLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, req.CreditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(insertToLegQuery).
			WithArgs(
				uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, req.CreditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
			).WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(linkLegsQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(adjustBalanceQuery).WithArgs(debitAmount, req.FromAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(adjustBalanceQuery).WithArgs(req.CreditAmount, req.ToAccountID).
			WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}

// TestDeleteTransferWithBalance covers removing both legs of a transfer and reversing their balance
// effects, atomically — plus the single-leg edge case where the paired leg is already gone.
func TestDeleteTransferWithBalance(t *testing.T) {
	t.Parallel()

	id := fakeID()
	accountID, amount := fakeID(), fakeAmount()
	otherID, otherAccountID, otherAmount := fakeID(), fakeID(), fakeAmount()

	t.Run("removes both legs and reverses both balances", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(transferLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, otherID),
		)
		mock.ExpectQuery(otherLegLookupQuery).WithArgs(otherID).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(otherAccountID, otherAmount),
		)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(reverseBalanceQuery).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(deleteTxQuery).WithArgs(otherID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(reverseBalanceQuery).WithArgs(otherAmount, otherAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("a paired leg that's already gone is skipped, only one leg removed", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(transferLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, otherID),
		)
		mock.ExpectQuery(otherLegLookupQuery).WithArgs(otherID).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(reverseBalanceQuery).WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(transferLookupQuery).WithArgs(id).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("insufficient balance reversing a leg's account rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(transferLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, nil),
		)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(reverseBalanceQuery).WithArgs(amount, accountID).WillReturnError(mysqlErr(errDataOutOfRange))
		mock.ExpectRollback()

		_, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("a driver error deleting a leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)
		mock.ExpectBegin()
		mock.ExpectQuery(transferLookupQuery).WithArgs(id).WillReturnRows(
			sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).AddRow(accountID, amount, nil),
		)
		mock.ExpectExec(deleteTxQuery).WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})
}
