package cmd

import (
	"bytes"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAuthTokenPrintsAPIKey(t *testing.T) {
	var output bytes.Buffer
	command := newRootCommand(environment{
		config: config.Config{APIKey: "my-api-key"},
	})
	command.SetOut(&output)
	command.SetArgs([]string{"auth", "token"})

	require.NoError(t, command.Execute())
	assert.Equal(t, "my-api-key\n", output.String())
}

func TestAuthTokenRejectsMissingAPIKey(t *testing.T) {
	command := newRootCommand(environment{})
	command.SetArgs([]string{"auth", "token"})

	err := command.Execute()

	require.EqualError(t, err, "no Requesty API key configured")
}

func TestAuthCommandIsHidden(t *testing.T) {
	command := newRootCommand(environment{})

	auth, _, err := command.Find([]string{"auth"})

	require.NoError(t, err)
	assert.True(t, auth.Hidden)
}
