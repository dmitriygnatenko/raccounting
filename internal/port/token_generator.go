package port

//go:generate go tool mockgen -source=token_generator.go -destination=mocks/token_generator_mock.go -package=mocks

// TokenGenerator produces unguessable session tokens.
type TokenGenerator interface {
	NewToken() (string, error)
}
