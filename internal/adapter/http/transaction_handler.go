package http

import (
	"net/http"
	"strconv"
	"time"

	"raccounting/internal/domain/entity"
	"raccounting/internal/domain/usecase/transaction/create"
	"raccounting/internal/domain/usecase/transaction/delete"
	"raccounting/internal/domain/usecase/transaction/list"
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

// transactionListResponse is GET /api/transactions's wire shape — the requested page, plus enough
// to paginate (page/pageSize/totalCount/totalPages) and to show an accurate total for the whole
// filtered set (sumsByCurrency), not just the page.
type transactionListResponse struct {
	Transactions   []entity.Transaction `json:"transactions"`
	Page           int                  `json:"page"`
	PageSize       int                  `json:"pageSize"`
	TotalCount     int                  `json:"totalCount"`
	TotalPages     int                  `json:"totalPages"`
	SumsByCurrency map[string]int64     `json:"sumsByCurrency"`
}

// parseOptionalDate parses a "YYYY-MM-DD" query param, returning nil if raw is empty. ok is false
// (with a 400 already written) if raw is non-empty but malformed.
func parseOptionalDate(w http.ResponseWriter, raw, field string) (*time.Time, bool) {
	if raw == "" {
		return nil, true
	}

	t, err := time.Parse(entity.DateLayout, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Invalid "+field)
		return nil, false
	}

	return &t, true
}

// parseOptionalUint64 parses a positive-integer query param, returning nil if raw is empty. ok is
// false (with a 400 already written) if raw is non-empty but malformed.
func parseOptionalUint64(w http.ResponseWriter, raw, field string) (*uint64, bool) {
	if raw == "" {
		return nil, true
	}

	v, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || v == 0 {
		writeError(w, http.StatusBadRequest, "Invalid "+field)
		return nil, false
	}

	return &v, true
}

// handleListTransactions handles GET /api/transactions?dateFrom=&dateTo=&accountId=&categoryId=&tagId=&type=&search=&page=&pageSize=.
// Every filter is optional; omitting a bound returns every transaction on that dimension.
func (s *Server) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	query := r.URL.Query()

	dateFrom, ok := parseOptionalDate(w, query.Get("dateFrom"), "dateFrom")
	if !ok {
		return
	}

	dateTo, ok := parseOptionalDate(w, query.Get("dateTo"), "dateTo")
	if !ok {
		return
	}

	accountID, ok := parseOptionalUint64(w, query.Get("accountId"), "accountId")
	if !ok {
		return
	}

	categoryID, ok := parseOptionalUint64(w, query.Get("categoryId"), "categoryId")
	if !ok {
		return
	}

	tagID, ok := parseOptionalUint64(w, query.Get("tagId"), "tagId")
	if !ok {
		return
	}

	var transactionType *entity.TransactionType
	if raw := query.Get("type"); raw != "" {
		v, err := strconv.ParseUint(raw, 10, 8)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid type")
			return
		}

		t := entity.TransactionType(v)
		transactionType = &t
	}

	page, _ := strconv.Atoi(query.Get("page"))
	pageSize, _ := strconv.Atoi(query.Get("pageSize"))

	out, err := s.Transactions.List.Execute(r.Context(), list.Input{
		DateFrom:   dateFrom,
		DateTo:     dateTo,
		AccountID:  accountID,
		CategoryID: categoryID,
		TagID:      tagID,
		Type:       transactionType,
		Search:     query.Get("search"),
		Page:       page,
		PageSize:   pageSize,
	})
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, transactionListResponse{
		Transactions:   out.Transactions,
		Page:           out.Page,
		PageSize:       out.PageSize,
		TotalCount:     out.TotalCount,
		TotalPages:     out.TotalPages,
		SumsByCurrency: out.SumsByCurrency,
	})
}

// handleTransactionUsage handles GET /api/transactions/usage.
func (s *Server) handleTransactionUsage(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Transactions.Usage.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"accountIds":  out.AccountIDs,
		"categoryIds": out.CategoryIDs,
	})
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
