package port

// TokenGenerator produces unguessable session tokens.
type TokenGenerator interface {
	NewToken() (string, error)
}
