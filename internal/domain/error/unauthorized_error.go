package error

import "errors"

// UnauthorizedError signals missing or invalid credentials or session. Maps to 401 Unauthorized.
type UnauthorizedError struct {
	Message string
}

// Error implements the error interface.
func (e UnauthorizedError) Error() string {
	return e.Message
}

func IsUnauthorizedError(err error) bool {
	var target *UnauthorizedError
	return errors.As(err, &target)
}
