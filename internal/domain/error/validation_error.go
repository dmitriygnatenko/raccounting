package error

import (
	"errors"
	"maps"
	"slices"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// ValidationError signals that the caller-supplied input was rejected, before anything was changed.
//
// One of the two payloads is set. Message carries a ready-made sentence for input that is
// structurally fine but fails a referential check ("Currency not found"), while Fields carries the
// field name -> error map that validation.ValidateStruct produces for form-shaped input. Error()
// renders either one, so callers that only need text can ignore the difference.
type ValidationError struct {
	Message string
	Fields  map[string]error
}

// Error implements the error interface.
//
// Per-field errors are rendered ordered by field name, so the same input always produces the same
// message (Go map iteration order is random). A lone field error is returned verbatim, and only the
// multi-error case gets joined into "One. Two.".
func (e ValidationError) Error() string {
	messages := make([]string, 0, len(e.Fields))

	for _, field := range slices.Sorted(maps.Keys(e.Fields)) {
		fieldErr := e.Fields[field]
		if fieldErr == nil || fieldErr.Error() == "" {
			continue
		}

		messages = append(messages, fieldErr.Error())
	}

	switch len(messages) {
	case 0:
		return e.Message
	case 1:
		return messages[0]
	}

	for i, message := range messages {
		if !strings.HasSuffix(message, ".") {
			messages[i] = message + "."
		}
	}

	return strings.Join(messages, " ")
}

// ToValidationError converts an error returned by ozzo-validation (validation.Validate or
// validation.ValidateStruct) into a *ValidationError, so the rest of the code works with this
// package's error vocabulary instead of reaching into ozzo-validation's types itself.
//
// A nil error yields a nil *ValidationError, so the result must not be returned straight into an
// error result unless err is known to be non-nil: a typed nil pointer in an error interface is not
// nil. Assign it to a variable and nil-check that instead.
func ToValidationError(err error) *ValidationError {
	if err == nil {
		return nil
	}

	var fieldErrs validation.Errors
	if errors.As(err, &fieldErrs) {
		return &ValidationError{
			Fields: fieldErrs,
		}
	}

	return &ValidationError{
		Message: err.Error(),
	}
}

func IsValidationError(err error) bool {
	var target *ValidationError
	return errors.As(err, &target)
}
