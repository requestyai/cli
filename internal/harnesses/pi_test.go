package harnesses

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPiHarnessRoundTrip(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with Pi installed but not integrated.
	configDir := t.TempDir()
	modelsPath := filepath.Join(configDir, "models.json")
	require.NoError(t, os.WriteFile(modelsPath, []byte(`{
		"providers": {
			"other": {"name": "Other provider"}
		}
	}`), 0o600))

	harness := NewPiHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, modelsPath)
	assert.Equal(t, false, status.Configured)

	// Configure the machine.
	err = harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	})
	require.NoError(t, err)

	// Validate Pi is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)

	models, err := os.ReadFile(modelsPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"providers": {
			"other": {"name": "Other provider"},
			"requesty": {
				"name": "Requesty",
				"baseUrl": "https://router.requesty.ai",
				"api": "anthropic-messages",
				"apiKey": "my-api-key",
				"headers": {
					"HTTP-Referer": "https://pi.dev",
					"X-Title": "Pi"
				},
				"models": [
					{"id": "anthropic/claude-fable-5"}
				]
			}
		}
	}`, string(models))
}

func TestPiHarnessConfigureCreatesMissingConfig(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	modelsPath := filepath.Join(configDir, "models.json")
	harness := NewPiHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	}))

	models, err := os.ReadFile(modelsPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"providers": {
			"requesty": {
				"name": "Requesty",
				"baseUrl": "https://router.requesty.ai",
				"api": "anthropic-messages",
				"apiKey": "my-api-key",
				"headers": {
					"HTTP-Referer": "https://pi.dev",
					"X-Title": "Pi"
				},
				"models": [
					{"id": "anthropic/claude-fable-5"}
				]
			}
		}
	}`, string(models))
}

func TestPiHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirPi()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".pi", "agent"), configDir)
}
