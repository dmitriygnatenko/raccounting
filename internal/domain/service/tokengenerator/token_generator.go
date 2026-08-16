// Package tokengenerator implements port.TokenGenerator.
package tokengenerator

import (
	"crypto/rand"
	"encoding/hex"
)

// RandomTokenGenerator implements port.TokenGenerator using a CSPRNG.
type RandomTokenGenerator struct{}

// New builds a RandomTokenGenerator.
func New() *RandomTokenGenerator {
	return &RandomTokenGenerator{}
}

// NewToken returns a fresh 32-byte random token, hex-encoded.
func (g *RandomTokenGenerator) NewToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return hex.EncodeToString(b), nil
}
