package harnesses

import (
	"os"
	"path/filepath"
	"testing"

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
	authPath := filepath.Join(configDir, "auth.json")
	require.NoError(t, os.WriteFile(configPath, []byte("model = \"gpt-5.5\"\n"), 0o600))
	require.NoError(t, os.WriteFile(authPath, []byte(`{"auth_mode": "chatgpt"}`), 0o600))

	harness := NewCodexHarness(config, configDir)

	status, err := harness.Status()
	require.NoError(t, err)
	assert.Contains(t, status.Files, configPath)
	assert.Contains(t, status.Files, authPath)
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

func TestCodexHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirCodex()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".codex"), configDir)
}
