package port

//go:generate go tool mockgen -source=password_hasher.go -destination=mocks/password_hasher_mock.go -package=mocks

// PasswordHasher hashes and verifies passwords (bcrypt in production).
type PasswordHasher interface {
	Hash(password string) (string, error)
	Compare(hash, password string) bool
}
