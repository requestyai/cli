// Package oauth signs the user in to Requesty with the OAuth 2.1 authorization
// code flow: PKCE, a loopback redirect, and a form-encoded token exchange.
package oauth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
)

// NewVerifier returns a fresh PKCE code verifier: 32 random bytes encoded as
// 43 unpadded base64url characters (RFC 7636 Section 4.1).
func NewVerifier() (string, error) {
	return randomToken(32)
}

// Challenge derives the S256 code challenge for verifier (RFC 7636 Section 4.2).
func Challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

// NewState returns a random value that ties the authorization response back to
// the request that started it.
func NewState() (string, error) {
	return randomToken(16)
}

func randomToken(size int) (string, error) {
	raw := make([]byte, size)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("failed to read random bytes: %w", err)
	}

	return base64.RawURLEncoding.EncodeToString(raw), nil
}
