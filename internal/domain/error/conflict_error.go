package error

import "errors"

// ConflictError signals a request that can't be applied because of the current state of the data
// (duplicate username, an account/category still referenced by transactions, ...). Maps to 409
// Conflict.
type ConflictError struct {
	Message string
}

// Error implements the error interface.
func (e ConflictError) Error() string {
	return e.Message
}

func IsConflictError(err error) bool {
	var target *ConflictError
	return errors.As(err, &target)
}
