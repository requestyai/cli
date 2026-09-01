package harnesses

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestHermesHarnessRoundTrip(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with Hermes installed but not integrated.
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`model:
  default: "anthropic/claude-sonnet-4-5"
  provider: "other"
custom_providers:
  - name: other
    base_url: "https://api.other.ai"
    api_key: "keep-me"
`), 0o600))

	harness := NewHermesHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, configPath)
	assert.Equal(t, false, status.Configured)

	// Configure the machine.
	err = harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	})
	require.NoError(t, err)

	// Validate Hermes is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)

	settingsBytes, err := os.ReadFile(configPath)
	require.NoError(t, err)

	settings := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(settingsBytes, &settings))
	assert.Equal(t, map[string]any{
		"model": map[string]any{
			"default":  "anthropic/claude-fable-5",
			"provider": "requesty",
		},
		"custom_providers": []any{
			map[string]any{
				"name":     "other",
				"base_url": "https://api.other.ai",
				"api_key":  "keep-me",
			},
			map[string]any{
				"name":     "requesty",
				"base_url": "https://router.requesty.ai",
				"api_key":  "my-api-key",
				"api_mode": "anthropic_messages",
			},
		},
	}, settings)
}

func TestHermesHarnessUpdatesExistingProvider(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.eu.requesty.ai",
		APIKey:        "my-new-api-key",
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	require.NoError(t, os.WriteFile(configPath, []byte(`custom_providers:
  - name: requesty
    base_url: "https://router.requesty.ai"
    api_key: "my-old-api-key"
    api_mode: anthropic_messages
    timeout: 300
`), 0o600))

	harness := NewHermesHarness(config, configDir)
	require.NoError(t, harness.Configure(ConfigureOptions{Model: "anthropic/claude-fable-5"}))

	settingsBytes, err := os.ReadFile(configPath)
	require.NoError(t, err)

	settings := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(settingsBytes, &settings))
	assert.Equal(t, []any{
		map[string]any{
			"name":     "requesty",
			"base_url": "https://router.eu.requesty.ai",
			"api_key":  "my-new-api-key",
			"api_mode": "anthropic_messages",
			"timeout":  300,
		},
	}, settings["custom_providers"])

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)
}

func TestHermesHarnessConfigureCreatesMissingConfig(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.yaml")
	harness := NewHermesHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	}))

	configBytes, err := os.ReadFile(configPath)
	require.NoError(t, err)

	settings := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(configBytes, &settings))
	assert.Equal(t, map[string]any{
		"model": map[string]any{
			"default":  "anthropic/claude-fable-5",
			"provider": "requesty",
		},
		"custom_providers": []any{
			map[string]any{
				"name":     "requesty",
				"base_url": "https://router.requesty.ai",
				"api_key":  "my-api-key",
				"api_mode": "anthropic_messages",
			},
		},
	}, settings)
}

func TestHermesHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirHermes()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".hermes"), configDir)
}
