package tokengenerator

import (
	"encoding/hex"
	"testing"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/stretchr/testify/require"
)

// TestNewToken checks that NewToken returns a 32-byte token hex-encoded to 64
// characters, with no error.
func TestNewToken(t *testing.T) {
	t.Parallel()

	g := New()

	token, err := g.NewToken()
	require.NoError(t, err)

	const wantLen = 64 // 32 bytes, hex-encoded

	require.Len(t, token, wantLen)

	_, err = hex.DecodeString(token)
	require.NoError(t, err)
}

// TestNewTokenUnique checks that repeated calls don't repeat tokens — the
// CSPRNG backing NewToken should never produce the same 32-byte value twice in a small sample.
func TestNewTokenUnique(t *testing.T) {
	t.Parallel()

	g := New()

	n := gofakeit.Number(500, 1500)
	seen := make(map[string]bool, n)

	for range n {
		token, err := g.NewToken()
		require.NoError(t, err)

		require.False(t, seen[token], "NewToken() produced duplicate token %q", token)
		seen[token] = true
	}
}
