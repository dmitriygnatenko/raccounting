package postgres

import (
	"context"

	"raccounting/internal/domain/entity"
	"raccounting/internal/storage/model"
)

// accountScanner is satisfied by both *sql.Row and *sql.Rows.
type accountScanner interface {
	Scan(dest ...any) error
}

// scanAccount reads the account column list (id, name, type, currency, balance, status), in the
// order every account query below selects it.
func scanAccount(row accountScanner) (model.Account, error) {
	var m model.Account

	if err := row.Scan(&m.ID, &m.Name, &m.Type, &m.CurrencyCode, &m.Balance, &m.Status); err != nil {
		return model.Account{}, err
	}

	return m, nil
}

const accountColumns = `id, name, type, currency, balance, status`

func archivedStatus(archived bool) entity.AccountStatus {
	if archived {
		return entity.AccountStatusArchived
	}

	return entity.AccountStatusActive
}

// ListAccounts returns every account, oldest-created first.
func (s *Storage) ListAccounts(ctx context.Context) ([]model.Account, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+accountColumns+` FROM accounts ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []model.Account

	for rows.Next() {
		m, err := scanAccount(rows)
		if err != nil {
			return nil, err
		}

		accounts = append(accounts, m)
	}

	return accounts, rows.Err()
}

// FindAccountByID returns an account by id. sql.ErrNoRows if it doesn't exist.
func (s *Storage) FindAccountByID(ctx context.Context, id uint64) (model.Account, error) {
	row := s.DB.QueryRowContext(ctx, `SELECT `+accountColumns+` FROM accounts WHERE id = $1`, id)

	return scanAccount(row)
}

// CreateAccount inserts an account row and returns its new id. A negative opening balance comes
// back wrapped in storageError.InsufficientBalanceError (accounts.balance has CHECK (balance >= 0)).
func (s *Storage) CreateAccount(ctx context.Context, req model.AccountCreateRequest) (uint64, error) {
	id, err := s.insertReturningID(ctx,
		`INSERT INTO accounts (name, type, currency, balance) VALUES ($1, $2, $3, $4) RETURNING id`,
		req.Name, uint8(req.Type), req.CurrencyCode, req.Balance,
	)
	if err != nil {
		return 0, wrapInsufficientBalance(err)
	}

	return id, nil
}

// UpdateAccount changes name/type/currency/status, returning the full updated row.
func (s *Storage) UpdateAccount(
	ctx context.Context, req model.AccountUpdateRequest,
) (model.Account, bool, error) {
	res, err := s.DB.ExecContext(ctx,
		`UPDATE accounts SET name = $1, type = $2, currency = $3, status = $4, updated_at = CURRENT_TIMESTAMP
		 WHERE id = $5`,
		req.Name, uint8(req.Type), req.CurrencyCode, uint8(archivedStatus(req.Archived)), req.ID,
	)
	if err != nil {
		return model.Account{}, false, err
	}

	found, err := affected(res)
	if err != nil || !found {
		return model.Account{}, false, err
	}

	row, err := s.FindAccountByID(ctx, req.ID)

	return row, true, err
}

// DeleteAccount removes an account row. found is false if no account with this id existed. A
// FOREIGN KEY violation (still referenced by a transaction) comes back wrapped in
// storageError.ForeignKeyViolationError.
func (s *Storage) DeleteAccount(ctx context.Context, id uint64) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM accounts WHERE id = $1`, id)
	if err != nil {
		return false, wrapForeignKey(err)
	}

	return affected(res)
}
