package app

import (
	"raccounting/internal/domain/service/passwordhasher"
	"raccounting/internal/domain/service/tokengenerator"
)

// services bundles the domain services that are shared across multiple use cases (e.g. hasher is
// needed by login and the credentials use case alike), so each is constructed once here rather than
// once per use case.
type services struct {
	Hasher *passwordhasher.BcryptHasher
	Tokens *tokengenerator.RandomTokenGenerator
}

// newServices constructs the shared domain services. They're all stateless/self-contained, so this
// never fails.
func newServices() services {
	return services{
		Hasher: passwordhasher.New(),
		Tokens: tokengenerator.New(),
	}
}
