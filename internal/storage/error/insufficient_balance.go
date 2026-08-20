package error

import "errors"

// InsufficientBalanceError reports that a write would have taken an account's balance below zero.
// accounts.balance is unsigned (MySQL: UNSIGNED; SQLite/Postgres, which have no unsigned integer
// type: CHECK (balance >= 0)), so this is what every driver adapter (internal/adapter/mysql,
// postgres, sqlite) translates that violation into, in its own dialect. The repository that
// recognizes it with errors.Is is what turns it into a message-less domain *ConflictError, leaving
// the use case to supply the wording ("overdraw", "opening balance can't be negative", ...).
var InsufficientBalanceError = errors.New("insufficient balance")
