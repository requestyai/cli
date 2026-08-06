package harnesses

import (
	"os"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexIntegrationRoundTrip(t *testing.T) {
	config := config.Config{
		BaseURL: "https://router.requesty.ai",
		APIKey:  "my-api-key",
	}

	harness := NewCodexHarness(config)

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
		Model: "openai-responses/gpt-5.5",
	})
	require.NoError(t, err)

	// Validate Codex is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)
}
