package authenticate

import "raccounting/internal/domain/entity"

// Output is the user a session token resolved to.
type Output struct {
	User entity.PublicUser
}
