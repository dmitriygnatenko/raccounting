package http

import (
	"net/http"

	domainerror "raccounting/internal/domain/error"
)

// writeUseCaseError maps a use case error onto an HTTP response: the use case builds the exact
// user-facing message, this only picks the status code from the error's type. An error of no known
// domain type is an internal failure and becomes a 500.
func writeUseCaseError(w http.ResponseWriter, err error) {
	if domainerror.IsValidationError(err) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if domainerror.IsNotFoundError(err) {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if domainerror.IsConflictError(err) {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	if domainerror.IsUnauthorizedError(err) {
		writeError(w, http.StatusUnauthorized, err.Error())
		return
	}

	writeError(w, http.StatusInternalServerError, err.Error())
}
