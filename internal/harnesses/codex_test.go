package harnesses

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCodexIntegrationRoundTrip(t *testing.T) {
	cfg := config.Config{
		Name:          "work",
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}

	// Simulate a machine with Codex installed but not integrated.
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.toml")
	require.NoError(t, os.WriteFile(configPath, []byte("model = \"gpt-5.5\"\n"), 0o600))

	harness := newCodexHarness(cfg, configDir)

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
	cfg := config.Config{
		Name:          "work",
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.toml")
	harness := newCodexHarness(cfg, configDir)

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
		Args:    []string{"auth", "token", "--profile", "work"},
	}, parsedConfig.ModelProviders[codexModelProvider].Auth)
	_, err = os.Stat(filepath.Join(configDir, "auth.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestCodexHarnessConfigureOverwriteUsesProviderAuth(t *testing.T) {
	cfg := config.Config{
		Name:          "work",
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	harness := newCodexHarness(cfg, configDir)

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
		Args:    []string{"auth", "token", "--profile", "work"},
	}, parsedConfig.ModelProviders[codexModelProvider].Auth)
	_, err = os.Stat(filepath.Join(configDir, "auth.json"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

// codexOverrides collects the `-c key=value` pairs from a launch and checks
// each value parses the way Codex parses it: as the right-hand side of a TOML
// assignment.
func codexOverrides(t *testing.T, args []string) map[string]any {
	t.Helper()

	overrides := map[string]any{}
	for i := 0; i+1 < len(args); i++ {
		if args[i] != "-c" {
			continue
		}
		key, value, found := strings.Cut(args[i+1], "=")
		require.True(t, found, "override %q has no '='", args[i+1])

		var parsed struct {
			Value any `toml:"value"`
		}
		require.NoError(t, toml.Unmarshal([]byte("value = "+value), &parsed), "override %q is not valid TOML", args[i+1])
		overrides[key] = parsed.Value
	}

	return overrides
}

func TestTomlStringQuotesForCodexOverrides(t *testing.T) {
	assert.Equal(t, `'plain'`, tomlString("plain"))
	assert.Equal(t, `'/Users/me/My Tools/requesty'`, tomlString("/Users/me/My Tools/requesty"))
	assert.Equal(t, `"it's"`, tomlString("it's"))

	for _, value := range []string{"plain", "/Users/me/My Tools/requesty", "it's", `C:\Program Files\requesty.exe`} {
		var parsed struct {
			Value string `toml:"value"`
		}
		require.NoError(t, toml.Unmarshal([]byte("value = "+tomlString(value)), &parsed))
		assert.Equal(t, value, parsed.Value)
	}
}
