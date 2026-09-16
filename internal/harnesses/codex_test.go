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

func TestCodexHarnessLaunchInjectsRequestyOverrides(t *testing.T) {
	binaryPath := fakeBinary(t, "codex")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewCodexHarness(cfg, t.TempDir())

	err := harness.Launch(LaunchOptions{
		Model:  "openai/gpt-5",
		Effort: EffortXHigh,
		Args:   []string{"--full-auto", "fix the tests"},
		Env:    []string{"HOME=/home/me"},
	})
	require.NoError(t, err)

	assert.Equal(t, binaryPath, captured.path)
	assert.Equal(t, "codex", captured.argv[0])
	assert.Equal(t, []string{"HOME=/home/me"}, captured.env, "the key travels through auth.command, not the environment")

	overrides := codexOverrides(t, captured.argv)
	assert.Equal(t, "requesty", overrides["model_provider"])
	assert.Equal(t, "Requesty", overrides["model_providers.requesty.name"])
	assert.Equal(t, "https://router.requesty.ai/v1", overrides["model_providers.requesty.base_url"])
	assert.Equal(t, "OpenAI Codex", overrides["model_providers.requesty.http_headers.X-Title"])
	assert.Equal(t, []any{"auth", "token"}, overrides["model_providers.requesty.auth.args"])
	assert.Equal(t, false, overrides["model_supports_reasoning_summaries"])
	assert.Equal(t, "xhigh", overrides["model_reasoning_effort"])

	authCommand, _ := overrides["model_providers.requesty.auth.command"].(string)
	executable, err := os.Executable()
	require.NoError(t, err)
	assert.Equal(t, executable, authCommand, "auth.command points at this very binary")

	// Model, effort and then the user's own arguments come after the overrides, in order.
	tail := captured.argv[len(captured.argv)-6:]
	assert.Equal(t, []string{"-m", "openai/gpt-5", "-c", "model_reasoning_effort='xhigh'", "--full-auto", "fix the tests"}, tail)
	assert.NotContains(t, strings.Join(captured.argv, " "), "my-api-key")
}

func TestCodexHarnessLaunchWithoutOverridesOmitsModelAndEffort(t *testing.T) {
	fakeBinary(t, "codex")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewCodexHarness(cfg, t.TempDir())

	require.NoError(t, harness.Launch(LaunchOptions{Env: []string{}}))

	assert.NotContains(t, captured.argv, "-m")
	_, hasEffort := codexOverrides(t, captured.argv)["model_reasoning_effort"]
	assert.False(t, hasEffort)
}

func TestCodexHarnessLaunchRejectsUnsupportedEffort(t *testing.T) {
	fakeBinary(t, "codex")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewCodexHarness(cfg, t.TempDir())

	err := harness.Launch(LaunchOptions{Effort: EffortMax, Env: []string{}})

	require.EqualError(t, err, `Codex does not support reasoning effort "max" (accepted: minimal, low, medium, high, xhigh)`)
	assert.Empty(t, captured.path, "nothing should be launched")
}

func TestCodexHarnessLaunchRequiresBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewCodexHarness(cfg, t.TempDir())

	err := harness.Launch(LaunchOptions{Env: []string{}})

	require.EqualError(t, err, "`codex` is not on PATH; install Codex (https://developers.openai.com/codex/cli) and try again")
	assert.Empty(t, captured.path)
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

func TestCodexHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirCodex()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".codex"), configDir)
}
