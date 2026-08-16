package list

import "raccounting/internal/domain/entity"

// Output is every account belonging to the signed-in user.
type Output struct {
	Accounts []entity.Account
}
