package oauth

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChallengeMatchesRFC7636Vector(t *testing.T) {
	// RFC 7636 Appendix B.
	challenge := Challenge("dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk")

	assert.Equal(t, "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM", challenge)
}

func TestNewVerifierIsUnpaddedBase64URLOf32Bytes(t *testing.T) {
	verifier, err := NewVerifier()
	require.NoError(t, err)

	assert.Len(t, verifier, 43)
	raw, err := base64.RawURLEncoding.DecodeString(verifier)
	require.NoError(t, err)
	assert.Len(t, raw, 32)
}

func TestNewVerifierIsRandom(t *testing.T) {
	first, err := NewVerifier()
	require.NoError(t, err)
	second, err := NewVerifier()
	require.NoError(t, err)

	assert.NotEqual(t, first, second)
}

func TestNewStateIsUnpaddedBase64URLOf16Bytes(t *testing.T) {
	state, err := NewState()
	require.NoError(t, err)

	raw, err := base64.RawURLEncoding.DecodeString(state)
	require.NoError(t, err)
	assert.Len(t, raw, 16)
}
