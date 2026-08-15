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

func TestDeepSeekHarnessRoundTrip(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with DeepSeek Harness installed but not integrated.
	configDir := t.TempDir()
	settingsPath := filepath.Join(configDir, "settings.yaml")
	credentialsPath := filepath.Join(configDir, ".credentials.yaml")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`agent-default-model:
  provider: deepseek-official
  model: deepseek-v4-pro
llm-deepseek:
  apiKeyEnv: DEEPSEEK_API_KEY
`), 0o600))
	require.NoError(t, os.WriteFile(credentialsPath, []byte("DEEPSEEK_API_KEY: keep-me\n"), 0o600))

	harness := NewDeepSeekHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, settingsPath)
	assert.Contains(t, status.Files, credentialsPath)
	assert.Equal(t, false, status.Configured)

	// Configure the machine.
	err = harness.Configure(ConfigureOptions{
		Model: "deepseek/deepseek-v4-pro-0813",
	})
	require.NoError(t, err)

	// Validate DeepSeek Harness is integrated with Requesty.
	status, err = harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)

	settingsBytes, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	settings := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(settingsBytes, &settings))
	assert.Equal(t, map[string]any{
		"agent-default-model": map[string]any{
			"provider": "requesty",
			"model":    "deepseek/deepseek-v4-pro-0813",
		},
		"llm-deepseek": map[string]any{
			"apiKeyEnv": "DEEPSEEK_API_KEY",
		},
		"llm-pi-ai": map[string]any{
			"providers": map[string]any{
				"requesty": map[string]any{
					"displayName":          "Requesty",
					"apiKeyEnv":            "REQUESTY_API_KEY",
					"api":                  "openai-completions",
					"baseURL":              "https://router.requesty.ai/v1",
					"defaultContextWindow": 200000,
					"defaultMaxTokens":     8192,
					"headers": map[string]any{
						"HTTP-Referer": "https://requesty.ai",
						"X-Title":      "DeepSeek Harness",
					},
					"models": []any{
						map[string]any{"id": "deepseek/deepseek-v4-pro-0813"},
					},
				},
			},
		},
	}, settings)

	// The harness reads keys by name, so other stored credentials survive.
	credentialsBytes, err := os.ReadFile(credentialsPath)
	require.NoError(t, err)

	credentials := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(credentialsBytes, &credentials))
	assert.Equal(t, map[string]any{
		"DEEPSEEK_API_KEY": "keep-me",
		"REQUESTY_API_KEY": "my-api-key",
	}, credentials)

	// The harness rejects a credentials file other users can read.
	info, err := os.Stat(credentialsPath)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestDeepSeekHarnessConfigureCreatesMissingFiles(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.eu.requesty.ai",
		APIKey:        "my-api-key",
	}

	configDir := t.TempDir()
	harness := NewDeepSeekHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	}))

	settingsBytes, err := os.ReadFile(filepath.Join(configDir, "settings.yaml"))
	require.NoError(t, err)

	var settings deepseekSettings
	require.NoError(t, yaml.Unmarshal(settingsBytes, &settings))
	assert.Equal(t, "requesty", settings.DefaultModel.Provider)
	assert.Equal(t, "anthropic/claude-fable-5", settings.DefaultModel.Model)
	assert.Equal(t, deepseekProviderConfig{
		DisplayName:          "Requesty",
		APIKeyEnv:            "REQUESTY_API_KEY",
		API:                  "openai-completions",
		BaseURL:              "https://router.eu.requesty.ai/v1",
		DefaultContextWindow: 200000,
		DefaultMaxTokens:     8192,
		Headers: map[string]string{
			"HTTP-Referer": "https://requesty.ai",
			"X-Title":      "DeepSeek Harness",
		},
		Models: []deepseekModelConfig{
			{ID: "anthropic/claude-fable-5"},
		},
	}, settings.PiAI.Providers["requesty"])

	credentialsBytes, err := os.ReadFile(filepath.Join(configDir, ".credentials.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "REQUESTY_API_KEY: my-api-key\n", string(credentialsBytes))

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)
}

func TestDeepSeekHarnessConfigureOverwriteReplacesSettings(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	configDir := t.TempDir()
	settingsPath := filepath.Join(configDir, "settings.yaml")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`llm-deepseek:
  apiKeyEnv: DEEPSEEK_API_KEY
`), 0o600))

	harness := NewDeepSeekHarness(config, configDir)
	require.NoError(t, harness.Configure(ConfigureOptions{
		Model:     "deepseek/deepseek-v4-pro-0813",
		Overwrite: true,
	}))

	settingsBytes, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	settings := make(map[string]any)
	require.NoError(t, yaml.Unmarshal(settingsBytes, &settings))
	assert.NotContains(t, settings, "llm-deepseek")

	// The replaced settings are kept in the backup.
	backupBytes, err := os.ReadFile(settingsPath + ".requesty.bak")
	require.NoError(t, err)
	assert.Contains(t, string(backupBytes), "DEEPSEEK_API_KEY")

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Equal(t, true, status.Configured)
}

func TestDeepSeekHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	t.Setenv("DSH_HOME", "")

	configDir, err := DefaultConfigDirDeepSeek()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".dsh"), configDir)
}

func TestDeepSeekHarnessDefaultConfigDirFromEnvironment(t *testing.T) {
	t.Setenv("DSH_HOME", filepath.Join("custom", "harness-home"))

	configDir, err := DefaultConfigDirDeepSeek()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join("custom", "harness-home"), configDir)
}
