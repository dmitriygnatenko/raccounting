// Package passwordhasher implements port.PasswordHasher.
package passwordhasher

import "golang.org/x/crypto/bcrypt"

// BcryptHasher implements port.PasswordHasher using bcrypt.
type BcryptHasher struct{}

// New builds a BcryptHasher.
func New() *BcryptHasher {
	return &BcryptHasher{}
}

// Hash bcrypt-hashes a plaintext password.
func (h *BcryptHasher) Hash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}

	return string(hash), nil
}

// Compare reports whether password matches the given bcrypt hash.
func (h *BcryptHasher) Compare(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}
