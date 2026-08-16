package list

import "raccounting/internal/domain/entity"

// Output is every currency belonging to the signed-in user.
type Output struct {
	Currencies []entity.Currency
}
