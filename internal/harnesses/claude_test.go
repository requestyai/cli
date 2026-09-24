package harnesses

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaudeHarnessRoundTrip(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with Claude Code installed but not integrated.
	configDir := t.TempDir()
	settingsPath := filepath.Join(configDir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{"theme": "dark"}`), 0o600))

	harness := NewClaudeHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, settingsPath)
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

func TestClaudeHarnessConfigureCreatesMissingConfig(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	settingsPath := filepath.Join(configDir, "settings.json")
	harness := NewClaudeHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	}))

	settings, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"env": {
			"ANTHROPIC_BASE_URL": "https://router.requesty.ai",
			"ANTHROPIC_AUTH_TOKEN": "my-api-key",
			"ANTHROPIC_MODEL": "anthropic/claude-fable-5"
		}
	}`, string(settings))
}

func TestClaudeHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirClaudeCode()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".claude"), configDir)
}
