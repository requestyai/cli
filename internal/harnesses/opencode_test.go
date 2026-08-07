package harnesses

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOpenCodeHarnessRoundTrip(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with OpenCode installed but not integrated.
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "opencode.json")
	require.NoError(t, os.WriteFile(configPath, []byte(`{"theme": "system"}`), 0o600))

	harness := NewOpenCodeHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, configPath)
	assert.Equal(t, false, status.Configured)

	// Configure the machine.
	err = harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	})
	require.NoError(t, err)

	// Validate OpenCode is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)

	settings, err := os.ReadFile(configPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"$schema": "https://opencode.ai/config.json",
		"theme": "system",
		"model": "requesty/anthropic/claude-fable-5",
		"provider": {
			"requesty": {
				"options": {
					"baseURL": "https://router.requesty.ai/v1",
					"apiKey": "my-api-key",
					"headers": {"X-Title": "OpenCode"}
				}
			}
		}
	}`, string(settings))
}

func TestOpenCodeHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirOpenCode()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".config", "opencode"), configDir)
}
