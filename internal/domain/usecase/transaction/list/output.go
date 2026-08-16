package list

import "raccounting/internal/domain/entity"

// Output is every transaction belonging to the signed-in user.
type Output struct {
	Transactions []entity.Transaction
}
