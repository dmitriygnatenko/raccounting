package error

import "errors"

// InsufficientBalanceError reports that a write would have taken an account's balance below zero.
// accounts.balance is unsigned (MySQL: UNSIGNED; SQLite/Postgres, which have no unsigned integer
// type: CHECK (balance >= 0)), so this is what every adapter translates that violation into. The
// repository that recognizes it with errors.Is is what turns it into a domain *ConflictError.
var InsufficientBalanceError = errors.New("insufficient balance")
