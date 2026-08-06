package harnesses

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexIntegrationRoundTrip(t *testing.T) {
	harness := NewCodexHarness()

	// Validate not integrated on the github runner.
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, homePath+"/.codex/config.toml")
	assert.Contains(t, status.Files, homePath+"/.codex/auth.json")
	assert.Equal(t, false, status.Configured)

	// Configure the machine.
	err = harness.Configure(ConfigureOptions{
		APIKey: "my-api-key",
		Model:  "openai-responses/gpt-5.5",
	})
	require.NoError(t, err)

	// Validate Codex is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)
}
