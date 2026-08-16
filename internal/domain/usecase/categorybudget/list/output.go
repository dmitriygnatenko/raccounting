package list

import "raccounting/internal/domain/entity"

// Output is every category budget belonging to the signed-in user.
type Output struct {
	CategoryBudgets []entity.Budget
}
