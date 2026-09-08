package harnesses

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexIntegrationRoundTrip(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with Codex installed but not integrated.
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("model = \"gpt-5.5\"\n"), 0o600))

	harness := NewCodexHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, configPath)
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

func TestCodexHarnessConfigureCreatesMissingConfig(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.toml")
	harness := NewCodexHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "openai-responses/gpt-5.5",
	}))

	configBytes, err := os.ReadFile(configPath)
	require.NoError(t, err)
	var parsedConfig codexConfig
	require.NoError(t, toml.Unmarshal(configBytes, &parsedConfig))
	assert.Equal(t, "openai-responses/gpt-5.5", parsedConfig.Model)
	assert.Equal(t, codexModelProvider, parsedConfig.ModelProvider)
	assert.Equal(t, "https://router.requesty.ai/v1", parsedConfig.ModelProviders[codexModelProvider].BaseURL)
	assert.Equal(t, codexProviderAuth{
		Command: "requesty",
		Args:    []string{"auth", "token"},
	}, parsedConfig.ModelProviders[codexModelProvider].Auth)
	_, err = os.Stat(filepath.Join(configDir, "auth.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCodexHarnessConfigureOverwriteUsesProviderAuth(t *testing.T) {
	cfg := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	harness := NewCodexHarness(cfg, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model:     "openai-responses/gpt-5.5",
		Overwrite: true,
	}))

	configBytes, err := os.ReadFile(filepath.Join(configDir, "config.toml"))
	require.NoError(t, err)
	var parsedConfig codexConfig
	require.NoError(t, toml.Unmarshal(configBytes, &parsedConfig))
	assert.Equal(t, codexProviderAuth{
		Command: "requesty",
		Args:    []string{"auth", "token"},
	}, parsedConfig.ModelProviders[codexModelProvider].Auth)
	_, err = os.Stat(filepath.Join(configDir, "auth.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCodexHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirCodex()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".codex"), configDir)
}
