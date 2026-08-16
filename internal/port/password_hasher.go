package port

// PasswordHasher hashes and verifies passwords (bcrypt in production).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}
