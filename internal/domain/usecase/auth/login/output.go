package login

import (
	"raccounting/internal/domain/entity"
)

// Output is the signed-in account plus the session it was just started with.
type Output struct {
	User    entity.PublicUser
	Session entity.Session
}
