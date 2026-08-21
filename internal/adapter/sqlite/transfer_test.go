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

const (
	transferInsertFromQuery = `INSERT INTO transactions
		 (category_id, type, account_id, currency, amount, transfer_currency, transfer_amount,
		  transfer_rate, transfer_account_id, memo, operation_at)
		 VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	transferInsertToQuery = `INSERT INTO transactions
		 (category_id, type, account_id, currency, amount, transfer_transaction_id, transfer_currency,
		  transfer_amount, transfer_rate, transfer_account_id, memo, operation_at)
		 VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	transferLinkQuery          = `UPDATE transactions SET transfer_transaction_id = ? WHERE id = ?`
	transferBalanceUpdateQuery = `UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`
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

// TestCreateTransferWithBalance covers the happy path — both legs inserted, linked to each other,
// and both accounts' balances adjusted, all inside one transaction — plus each step's failure mode
// rolling back instead of leaving a half-written transfer.
func TestCreateTransferWithBalance(t *testing.T) {
	t.Parallel()

	req := fakeTransferCreateRequest()
	debitAmount := -req.Amount
	creditAmount := req.CreditAmount
	fromID, toID := int64(fakeID()), int64(fakeID())

	t.Run("inserts both legs, links them, adjusts both balances, and commits", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transferInsertFromQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, creditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(transferInsertToQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, creditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(transferLinkQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transferBalanceUpdateQuery).WithArgs(debitAmount, req.FromAccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transferBalanceUpdateQuery).WithArgs(creditAmount, req.ToAccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		got, err := s.CreateTransferWithBalance(context.Background(), req)
		require.NoError(t, err)

		fromIDU, toIDU := uint64(fromID), uint64(toID)
		require.Equal(t, fromIDU, got.LegFrom.ID)
		require.Equal(t, req.FromAccountID, got.LegFrom.AccountID)
		require.Equal(t, debitAmount, got.LegFrom.Amount)
		require.Equal(t, &toIDU, got.LegFrom.TransferTransactionID)
		require.Equal(t, toIDU, got.LegTo.ID)
		require.Equal(t, req.ToAccountID, got.LegTo.AccountID)
		require.Equal(t, creditAmount, got.LegTo.Amount)
		require.Equal(t, &fromIDU, got.LegTo.TransferTransactionID)
	})

	t.Run("a failure inserting the first leg rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transferInsertFromQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, creditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt).
			WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("a failure linking the legs rolls back", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transferInsertFromQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, creditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(transferInsertToQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, creditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(transferLinkQuery).WithArgs(toID, fromID).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("overdrawing the debited account rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transferInsertFromQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, creditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(transferInsertToQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, creditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(transferLinkQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transferBalanceUpdateQuery).WithArgs(debitAmount, req.FromAccountID).
			WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})

	t.Run("overdrawing the credited account rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectExec(transferInsertFromQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
				req.ToCurrencyCode, creditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(fromID, 1))
		mock.ExpectExec(transferInsertToQuery).
			WithArgs(uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, creditAmount,
				fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt).
			WillReturnResult(sqlmock.NewResult(toID, 1))
		mock.ExpectExec(transferLinkQuery).WithArgs(toID, fromID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transferBalanceUpdateQuery).WithArgs(debitAmount, req.FromAccountID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(transferBalanceUpdateQuery).WithArgs(creditAmount, req.ToAccountID).
			WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, err := s.CreateTransferWithBalance(context.Background(), req)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}

// TestDeleteTransferWithBalance covers removing both legs of a transfer and reversing their balance
// effects, the case where the paired leg has already been removed some other way, the not-found
// case, and the failure modes that have to roll back instead of leaving the transfer half-deleted.
func TestDeleteTransferWithBalance(t *testing.T) {
	t.Parallel()

	id := fakeID()
	otherID := fakeID()
	accountID, otherAccountID := fakeID(), fakeID()
	amount, otherAmount := fakeAmount(), fakeAmount()

	t.Run("deletes both legs and reverses both balances", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ?`).
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).
				AddRow(accountID, amount, otherID))
		mock.ExpectQuery(`SELECT account_id, amount FROM transactions WHERE id = ?`).
			WithArgs(otherID).
			WillReturnRows(sqlmock.NewRows([]string{"account_id", "amount"}).AddRow(otherAccountID, otherAmount))
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`).
			WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(otherID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`).
			WithArgs(otherAmount, otherAccountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("a paired leg that's already gone deletes just the one leg found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ?`).
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).
				AddRow(accountID, amount, otherID))
		mock.ExpectQuery(`SELECT account_id, amount FROM transactions WHERE id = ?`).
			WithArgs(otherID).WillReturnError(sql.ErrNoRows)
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`).
			WithArgs(amount, accountID).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectCommit()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.True(t, found)
	})

	t.Run("an unknown id is reported as not found", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ?`).
			WithArgs(id).WillReturnError(sql.ErrNoRows)
		mock.ExpectRollback()

		found, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.NoError(t, err)
		require.False(t, found)
	})

	t.Run("a driver error on the initial lookup is propagated", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ?`).
			WithArgs(id).WillReturnError(errStub)
		mock.ExpectRollback()

		_, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.ErrorIs(t, err, errStub)
	})

	t.Run("reversing a balance below zero rolls back and is an insufficient balance violation", func(t *testing.T) {
		t.Parallel()

		s, mock := newMock(t)

		mock.ExpectBegin()
		mock.ExpectQuery(`SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ?`).
			WithArgs(id).
			WillReturnRows(sqlmock.NewRows([]string{"account_id", "amount", "transfer_transaction_id"}).
				AddRow(accountID, amount, nil))
		mock.ExpectExec(`DELETE FROM transactions WHERE id = ?`).WithArgs(id).WillReturnResult(sqlmock.NewResult(0, 1))
		mock.ExpectExec(`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`).
			WithArgs(amount, accountID).WillReturnError(sqliteInsufficientBalanceErr())
		mock.ExpectRollback()

		_, err := s.DeleteTransferWithBalance(context.Background(), id)
		require.ErrorIs(t, err, storageError.InsufficientBalanceError)
	})
}
