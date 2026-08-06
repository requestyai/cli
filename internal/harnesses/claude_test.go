package harnesses

import (
	"os"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeHarnessRoundTrip(t *testing.T) {
	config := config.Config{
		BaseURL: "https://router.requesty.ai",
		APIKey:  "my-api-key",
	}

	harness := NewClaudeHarness(config)

	// Validate not integrated on the github runner.
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, homePath+"/.claude/settings.json")
	assert.Equal(t, false, status.Configured)

	// Configure the machine.
	err = harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	})
	require.NoError(t, err)

	// Validate Claude is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)
}
