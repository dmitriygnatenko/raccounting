package updatecredentials

import "raccounting/internal/domain/entity"

// Output is the account after the requested changes were applied.
type Output struct {
	User entity.PublicUser
}
