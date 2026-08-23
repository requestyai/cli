package util

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfirm(t *testing.T) {
	for _, answer := range []string{"y\n", "yes\n", " YES "} {
		var out bytes.Buffer
		confirmed, err := Confirm(strings.NewReader(answer), &out, "Continue? ")
		require.NoError(t, err)
		assert.True(t, confirmed)
		assert.Equal(t, "Continue? ", out.String())
	}
}

func TestConfirmDefaultsToNo(t *testing.T) {
	for _, answer := range []string{"\n", "no\n", ""} {
		var out bytes.Buffer
		confirmed, err := Confirm(strings.NewReader(answer), &out, "Continue? ")
		require.NoError(t, err)
		assert.False(t, confirmed)
		assert.Equal(t, "Continue? ", out.String())
	}
}
