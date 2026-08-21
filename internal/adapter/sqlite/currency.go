package sqlite

import (
	"context"
	"database/sql"

	"raccounting/internal/storage/model"
)

// dbtx is satisfied by both *sql.DB and *sql.Tx — it lets createCurrency/updateCurrency run either
// directly against the pool or inside the transaction that clears every other row's is_default flag.
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

const currencyColumns = `code, symbol, name, rate, is_default, status`

// ListCurrencies returns every currency, oldest-created first.
func (s *Storage) ListCurrencies(ctx context.Context) ([]model.Currency, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT `+currencyColumns+` FROM currencies ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var currencies []model.Currency

	for rows.Next() {
		var m model.Currency
		if err := rows.Scan(&m.Code, &m.Symbol, &m.Name, &m.Rate, &m.Default, &m.Status); err != nil {
			return nil, err
		}

		currencies = append(currencies, m)
	}

	return currencies, rows.Err()
}

// ExistsCurrency reports whether a currency with this code exists.
func (s *Storage) ExistsCurrency(ctx context.Context, code string) (bool, error) {
	var n int
	err := s.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM currencies WHERE code = ?`, code,
	).Scan(&n)

	return n > 0, err
}

// CreateCurrency inserts a currency row. A taken code comes back wrapped in
// storageError.UniqueViolationError. When req.Default is set, every other currency's is_default
// flag is cleared first, atomically, so at most one currency is ever the default.
func (s *Storage) CreateCurrency(ctx context.Context, req model.CurrencyCreateRequest) error {
	if !req.Default {
		return createCurrency(ctx, s.DB, req)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `UPDATE currencies SET is_default = 0`); err != nil {
		return err
	}

	if err = createCurrency(ctx, tx, req); err != nil {
		return err
	}

	return tx.Commit()
}

func createCurrency(ctx context.Context, db dbtx, req model.CurrencyCreateRequest) error {
	_, err := db.ExecContext(ctx,
		`INSERT INTO currencies (code, symbol, name, rate, is_default) VALUES (?, ?, ?, ?, ?)`,
		req.Code, req.Symbol, req.Name, req.Rate, req.Default,
	)

	return wrapUnique(err)
}

// UpdateCurrency changes symbol/name/rate/default/status, returning the full updated row. When
// req.Default is set, every other currency's is_default flag is cleared first, atomically, so at
// most one currency is ever the default.
func (s *Storage) UpdateCurrency(
	ctx context.Context, req model.CurrencyUpdateRequest,
) (model.Currency, bool, error) {
	if !req.Default {
		return updateCurrency(ctx, s.DB, req)
	}

	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return model.Currency{}, false, err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err = tx.ExecContext(ctx, `UPDATE currencies SET is_default = 0`); err != nil {
		return model.Currency{}, false, err
	}

	row, found, err := updateCurrency(ctx, tx, req)
	if err != nil || !found {
		return row, found, err
	}

	if err = tx.Commit(); err != nil {
		return model.Currency{}, false, err
	}

	return row, true, nil
}

func updateCurrency(ctx context.Context, db dbtx, req model.CurrencyUpdateRequest) (model.Currency, bool, error) {
	res, err := db.ExecContext(ctx,
		`UPDATE currencies SET symbol = ?, name = ?, rate = ?, is_default = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE code = ?`,
		req.Symbol, req.Name, req.Rate, req.Default, uint8(archivedStatus(req.Archived)), req.Code,
	)
	if err != nil {
		return model.Currency{}, false, err
	}

	found, err := affected(res)
	if err != nil || !found {
		return model.Currency{}, false, err
	}

	var m model.Currency

	err = db.QueryRowContext(ctx,
		`SELECT `+currencyColumns+` FROM currencies WHERE code = ?`, req.Code,
	).Scan(&m.Code, &m.Symbol, &m.Name, &m.Rate, &m.Default, &m.Status)

	return m, true, err
}

// DeleteCurrency removes a currency row. found is false if no currency with this code existed. A
// FOREIGN KEY violation (still referenced by an account) comes back wrapped in
// storageError.ForeignKeyViolationError.
func (s *Storage) DeleteCurrency(ctx context.Context, code string) (bool, error) {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM currencies WHERE code = ?`, code)
	if err != nil {
		return false, wrapForeignKey(err)
	}

	return affected(res)
}
