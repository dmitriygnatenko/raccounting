package http

import (
	"net/http"

	"raccounting/internal/domain/usecase/transfer/create"
	"raccounting/internal/domain/usecase/transfer/delete"
)

type transferRequest struct {
	FromAccountID uint64   `json:"fromAccountId"`
	ToAccountID   uint64   `json:"toAccountId"`
	Amount        int64    `json:"amount"`
	ToAmount      *int64   `json:"toAmount"`
	Rate          *float64 `json:"rate"`
	Date          string   `json:"date"`
	Memo          string   `json:"memo"`
}

type transferResponse struct {
	LegFrom any `json:"legFrom"`
	LegTo   any `json:"legTo"`
}

// handleCreateTransfer handles POST /api/transfers.
func (s *Server) handleCreateTransfer(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	var input transferRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	out, err := s.Transfers.Create.Execute(
		r.Context(),
		create.Input{
			FromAccountID: input.FromAccountID,
			ToAccountID:   input.ToAccountID,
			Amount:        input.Amount,
			ToAmount:      input.ToAmount,
			Rate:          input.Rate,
			Date:          input.Date,
			Memo:          input.Memo,
		},
	)
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusCreated, transferResponse{
		LegFrom: out.LegFrom,
		LegTo:   out.LegTo,
	})
}

// handleDeleteTransfer handles DELETE /api/transfers/{id} — id is either leg's own transaction id.
func (s *Server) handleDeleteTransfer(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	id, ok := parseIDParam(w, r, "id")
	if !ok {
		return
	}

	if err := s.Transfers.Delete.Execute(
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
