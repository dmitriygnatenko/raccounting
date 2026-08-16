package error

import "errors"

// NotFoundError signals that the requested record does not exist — or exists but belongs to another
// user, which the API deliberately reports the same way. Maps to 404 Not Found.
type NotFoundError struct {
	Message string
}

// Error implements the error interface.
func (e NotFoundError) Error() string {
	return e.Message
}

func IsNotFoundError(err error) bool {
	var target *NotFoundError
	return errors.As(err, &target)
}
