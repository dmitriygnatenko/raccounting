package error

import "errors"

// ForeignKeyViolationError reports that a write collided with a FOREIGN KEY constraint — a
// DELETE/UPDATE blocked by a row that still references this one (MySQL error 1451,
// ER_ROW_IS_REFERENCED_2; Postgres SQLSTATE 23503; SQLite's "FOREIGN KEY constraint failed"). Each
// driver adapter detects it in its own dialect and wraps it in this sentinel; the repository that
// recognizes it with errors.Is is what turns it into a message-less domain *ConflictError — the
// "can't delete what's in use" guard on accounts/categories/currencies still referenced by a
// transaction, worded by the use case.
var ForeignKeyViolationError = errors.New("foreign key constraint violation")
