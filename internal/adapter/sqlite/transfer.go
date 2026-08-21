package sqlite

import (
	"context"
	"database/sql"
	"errors"

	"raccounting/internal/domain/entity"
	"raccounting/internal/storage/model"
)

// CreateTransferWithBalance inserts both transfer legs — pointing at each other's id via
// transfer_transaction_id (debit leg negative amount, credit leg positive) — and adjusts both
// accounts' balances, atomically, in one DB transaction.
func (s *Storage) CreateTransferWithBalance(
	ctx context.Context, req model.TransferCreateRequest,
) (model.CreateTransferResult, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return model.CreateTransferResult{}, err
	}
	defer func() { _ = tx.Rollback() }()

	debitAmount := -req.Amount
	creditAmount := req.CreditAmount

	fromRes, err := tx.ExecContext(ctx,
		`INSERT INTO transactions
		 (category_id, type, account_id, currency, amount, transfer_currency, transfer_amount,
		  transfer_rate, transfer_account_id, memo, operation_at)
		 VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uint8(entity.TransactionTypeTransfer), req.FromAccountID, req.FromCurrencyCode, debitAmount,
		req.ToCurrencyCode, creditAmount, req.Rate, req.ToAccountID, req.Memo, req.OperationAt,
	)
	if err != nil {
		return model.CreateTransferResult{}, err
	}

	fromID, err := fromRes.LastInsertId()
	if err != nil {
		return model.CreateTransferResult{}, err
	}

	toRes, err := tx.ExecContext(ctx,
		`INSERT INTO transactions
		 (category_id, type, account_id, currency, amount, transfer_transaction_id, transfer_currency,
		  transfer_amount, transfer_rate, transfer_account_id, memo, operation_at)
		 VALUES (NULL, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		uint8(entity.TransactionTypeTransfer), req.ToAccountID, req.ToCurrencyCode, creditAmount,
		fromID, req.FromCurrencyCode, debitAmount, req.Rate, req.FromAccountID, req.Memo, req.OperationAt,
	)
	if err != nil {
		return model.CreateTransferResult{}, err
	}

	toID, err := toRes.LastInsertId()
	if err != nil {
		return model.CreateTransferResult{}, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE transactions SET transfer_transaction_id = ? WHERE id = ?`, toID, fromID,
	); err != nil {
		return model.CreateTransferResult{}, err
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		debitAmount, req.FromAccountID,
	); err != nil {
		return model.CreateTransferResult{}, wrapInsufficientBalance(err)
	}

	if _, err = tx.ExecContext(ctx,
		`UPDATE accounts SET balance = balance + ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		creditAmount, req.ToAccountID,
	); err != nil {
		return model.CreateTransferResult{}, wrapInsufficientBalance(err)
	}

	if err = tx.Commit(); err != nil {
		return model.CreateTransferResult{}, err
	}

	fromIDU := uint64(fromID)
	toIDU := uint64(toID)
	rate := req.Rate
	toCurrency := req.ToCurrencyCode
	fromCurrency := req.FromCurrencyCode

	legFrom := model.Transaction{
		ID:                    fromIDU,
		Type:                  uint8(entity.TransactionTypeTransfer),
		AccountID:             req.FromAccountID,
		CurrencyCode:          req.FromCurrencyCode,
		Amount:                debitAmount,
		TransferTransactionID: &toIDU,
		TransferCurrencyCode:  &toCurrency,
		TransferAmount:        &creditAmount,
		TransferRate:          &rate,
		TransferAccountID:     &req.ToAccountID,
		Memo:                  req.Memo,
		OperationAt:           req.OperationAt,
	}

	legTo := model.Transaction{
		ID:                    toIDU,
		Type:                  uint8(entity.TransactionTypeTransfer),
		AccountID:             req.ToAccountID,
		CurrencyCode:          req.ToCurrencyCode,
		Amount:                creditAmount,
		TransferTransactionID: &fromIDU,
		TransferCurrencyCode:  &fromCurrency,
		TransferAmount:        &debitAmount,
		TransferRate:          &rate,
		TransferAccountID:     &req.FromAccountID,
		Memo:                  req.Memo,
		OperationAt:           req.OperationAt,
	}

	return model.CreateTransferResult{LegFrom: legFrom, LegTo: legTo}, nil
}

// DeleteTransferWithBalance removes both legs of the transfer that id belongs to and reverses their
// balance effects, atomically, in one DB transaction. found is false if no transaction with this id
// existed.
//
// Unlike the MySQL/Postgres adapters, the leg lookup below doesn't use SELECT ... FOR UPDATE: SQLite
// doesn't support it, and the connection pool here is capped at one open connection (see
// storage.go), so this transaction already has exclusive access.
func (s *Storage) DeleteTransferWithBalance(ctx context.Context, id uint64) (bool, error) {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()

	var (
		accountID             uint64
		amount                int64
		transferTransactionID sql.NullInt64
	)

	err = tx.QueryRowContext(ctx,
		`SELECT account_id, amount, transfer_transaction_id FROM transactions WHERE id = ?`, id,
	).Scan(&accountID, &amount, &transferTransactionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}

		return false, err
	}

	type leg struct {
		id        uint64
		accountID uint64
		amount    int64
	}

	legs := []leg{{id: id, accountID: accountID, amount: amount}}

	if transferTransactionID.Valid {
		otherID := uint64(transferTransactionID.Int64)

		var (
			otherAccountID uint64
			otherAmount    int64
		)

		err = tx.QueryRowContext(ctx,
			`SELECT account_id, amount FROM transactions WHERE id = ?`, otherID,
		).Scan(&otherAccountID, &otherAmount)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return false, err
		}

		if err == nil {
			legs = append(legs, leg{id: otherID, accountID: otherAccountID, amount: otherAmount})
		}
	}

	for _, l := range legs {
		if _, err = tx.ExecContext(ctx, `DELETE FROM transactions WHERE id = ?`, l.id); err != nil {
			return false, err
		}

		if _, err = tx.ExecContext(ctx,
			`UPDATE accounts SET balance = balance - ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
			l.amount, l.accountID,
		); err != nil {
			return false, wrapInsufficientBalance(err)
		}
	}

	if err = tx.Commit(); err != nil {
		return false, err
	}

	return true, nil
}
