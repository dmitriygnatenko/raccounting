package http

import (
	"net/http"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase/transaction/create"
	"raccounting/internal/domain/usecase/transaction/delete"
	"raccounting/internal/domain/usecase/transaction/update"
)

// transactionRequest mirrors the frontend's transaction shape (see js/data/api.js
// createTransaction/updateTransaction): the client never sends "type" for a regular expense/income
// row — it's implied by the sign of amount. Transfers are created through the separate
// /api/transfers endpoint.
type transactionRequest struct {
	AccountID  uint64   `json:"accountId"`
	CategoryID *uint64  `json:"categoryId"`
	Memo       string   `json:"memo"`
	Amount     int64    `json:"amount"`
	Date       string   `json:"date"`
	TagIDs     []uint64 `json:"tagIds"`
}

// transactionType derives expense/income from the amount's sign, matching how the frontend decides
// direction (form.direction === 'expense' ? -Math.abs(amount) : Math.abs(amount)).
func transactionType(amount int64) uint8 {
	if amount < 0 {
		return uint8(entity.TransactionTypeExpense)
	}

	return uint8(entity.TransactionTypeIncome)
}

// handleListTransactions handles GET /api/transactions.
func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Transactions.List.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Transactions)
}

// handleCreateTransaction handles POST /api/transactions.
func (s *Server) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input transactionRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Transactions.Create.Execute(
		r.Context(),
		create.Input{
			AccountID:  input.AccountID,
			CategoryID: input.CategoryID,
			Type:       transactionType(input.Amount),
			Memo:       input.Memo,
			Amount:     input.Amount,
			Date:       input.Date,
			TagIDs:     input.TagIDs,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, out.Transaction)
}

// handleUpdateTransaction handles PUT /api/transactions/{id}.
func (s *Server) handleUpdateTransaction(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	var input transactionRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Transactions.Update.Execute(
		r.Context(),
		update.Input{
			ID:         id,
			AccountID:  input.AccountID,
			CategoryID: input.CategoryID,
			Type:       transactionType(input.Amount),
			Memo:       input.Memo,
			Amount:     input.Amount,
			Date:       input.Date,
			TagIDs:     input.TagIDs,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, out.Transaction)
}

// handleDeleteTransaction handles DELETE /api/transactions/{id}.
func (s *Server) handleDeleteTransaction(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.Transactions.Delete.Execute(
		r.Context(),
		delete.Input{
			ID: id,
		},
	); err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
