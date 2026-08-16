package error

import "errors"

// ForeignKeyViolationError reports that a write collided with a FOREIGN KEY constraint (MySQL error
// 1451, ER_ROW_IS_REFERENCED_2 — a DELETE/UPDATE blocked by a row that still references this one).
// The mysql adapter detects it and wraps it in this sentinel; the repository that recognizes it with
// errors.Is is what turns it into a domain *ConflictError — the "can't delete what's in use" guard
// on accounts/categories/currencies still referenced by a transaction.
var ForeignKeyViolationError = errors.New("foreign key constraint violation")
