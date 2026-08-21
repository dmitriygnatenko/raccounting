package list

import "raccounting/internal/domain/entity"

// Output is one page of transactions matching the Input filter, plus enough to paginate
// (TotalCount, TotalPages) and to show an accurate total for the whole filtered set — not just this
// page — via SumsByCurrency (see port.TransactionListResult).
type Output struct {
	Transactions   []entity.Transaction
	Page           int
	PageSize       int
	TotalCount     int
	TotalPages     int
	SumsByCurrency map[string]int64
}
