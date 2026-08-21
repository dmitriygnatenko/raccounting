package list

import (
	"time"

	"raccounting/internal/domain/entity"
)

// defaultPageSize and maxPageSize bound Input.PageSize: Execute defaults a zero PageSize to
// defaultPageSize, and clamps anything larger down to maxPageSize.
const (
	defaultPageSize = 100
	maxPageSize     = 1000
)

// Input narrows ListTransactions to one page of transactions matching every non-nil/non-empty
// field. DateFrom/DateTo are inclusive, date-only bounds.
type Input struct {
	DateFrom   *time.Time
	DateTo     *time.Time
	AccountID  *uint64
	CategoryID *uint64
	TagID      *uint64
	Type       *entity.TransactionType
	Search     string
	Page       int
	PageSize   int
}

// normalize defaults Page/PageSize and clamps PageSize into [1, maxPageSize].
func (i Input) normalize() Input {
	if i.Page < 1 {
		i.Page = 1
	}

	switch {
	case i.PageSize < 1:
		i.PageSize = defaultPageSize
	case i.PageSize > maxPageSize:
		i.PageSize = maxPageSize
	}

	return i
}
