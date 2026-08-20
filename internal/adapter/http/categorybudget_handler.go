package http

import (
	"net/http"
	"strconv"

	"raccounting/internal/domain/usecase/categorybudget/set"
)

// handleListCategoryBudgets handles GET /api/category-budgets. The flat list the use case returns
// is reshaped here into the nested {categoryId: {monthKey: amount}} map the frontend expects (see
// App.categoryBudgets in the mock).
func (s *Server) handleListCategoryBudgets(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	out, err := s.Budgets.List.Execute(r.Context())
	if err != nil {
		writeUseCaseError(w, err)
		return
	}

	budgets := make(map[string]map[string]int64, len(out.CategoryBudgets))

	for _, b := range out.CategoryBudgets {
		key := strconv.FormatUint(b.CategoryID, 10)
		if budgets[key] == nil {
			budgets[key] = map[string]int64{}
		}

		budgets[key][b.MonthKey] = b.Amount
	}

	writeJSON(w, http.StatusOK, budgets)
}

type setCategoryBudgetRequest struct {
	Amount int64 `json:"amount"`
}

// handleSetCategoryBudget handles PUT /api/category-budgets/{categoryId}/{monthKey}.
func (s *Server) handleSetCategoryBudget(w http.ResponseWriter, r *http.Request) {
	current := userFromContext(r)
	if current == nil {
		writeError(w, http.StatusUnauthorized, "Not authenticated")
		return
	}

	categoryID, err := strconv.ParseUint(r.PathValue("categoryId"), 10, 64)
	if err != nil || categoryID == 0 {
		writeError(w, http.StatusBadRequest, "Invalid category id")
		return
	}

	monthKey := r.PathValue("monthKey")

	var input setCategoryBudgetRequest
	if err = decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err = s.Budgets.Set.Execute(
		r.Context(),
		set.Input{
			CategoryID: categoryID,
			MonthKey:   monthKey,
			Amount:     input.Amount,
		},
	); err != nil {
		writeUseCaseError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
