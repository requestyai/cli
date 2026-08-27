package harnesses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/pelletier/go-toml/v2"
	"github.com/requestyai/cli/internal/attribution"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

const attributionTestModel = "anthropic/claude-sonnet-4-5"

// attributionSet holds one dimension of each kind: two that depend on where the
// harness runs, so every harness has to spell them its own way, and one that is
// known when the CLI runs and reaches every harness as a plain value.
func attributionSet() attribution.Set {
	return attribution.Set{
		{Header: "X-Requesty-Repo", Env: "REQUESTY_REPO", Shell: "echo repo"},
		{Header: "X-Requesty-Branch", Env: "REQUESTY_BRANCH", Shell: "echo branch"},
		{Header: "X-Requesty-User", Value: "ada"},
	}
}

func TestPiCarriesAttributionAsShellCommands(t *testing.T) {
	eachConfigurePath(t, func(t *testing.T, overwrite bool) {
		configDir := configureWithAttribution(t, func(config config.Config, dir string) Harness {
			return NewPiHarness(config, dir)
		}, overwrite)

		var models piModels
		readJSONFile(t, filepath.Join(configDir, "models.json"), &models)

		assert.Equal(t, map[string]string{
			"HTTP-Referer":      "https://pi.dev",
			"X-Title":           "Pi",
			"X-Requesty-Repo":   "!echo repo",
			"X-Requesty-Branch": "!echo branch",
			"X-Requesty-User":   "ada",
		}, models.Providers[piProvider].Headers)
	})
}

func TestOpenCodeCarriesAttributionAsEnvironmentPlaceholders(t *testing.T) {
	eachConfigurePath(t, func(t *testing.T, overwrite bool) {
		configDir := configureWithAttribution(t, func(config config.Config, dir string) Harness {
			return NewOpenCodeHarness(config, dir)
		}, overwrite)

		var settings openCodeConfig
		readJSONFile(t, filepath.Join(configDir, "opencode.json"), &settings)

		assert.Equal(t, map[string]string{
			"X-Title":           "OpenCode",
			"X-Requesty-Repo":   "{env:REQUESTY_REPO}",
			"X-Requesty-Branch": "{env:REQUESTY_BRANCH}",
			"X-Requesty-User":   "ada",
		}, settings.Providers[openCodeProvider].Options.Headers)
	})
}

func TestCodexCarriesAttributionAsEnvironmentHeaders(t *testing.T) {
	eachConfigurePath(t, func(t *testing.T, overwrite bool) {
		configDir := configureWithAttribution(t, func(config config.Config, dir string) Harness {
			return NewCodexHarness(config, dir)
		}, overwrite)

		var parsed codexConfig
		readTOMLFile(t, filepath.Join(configDir, "config.toml"), &parsed)
		provider := parsed.ModelProviders[codexModelProvider]

		assert.Equal(t, map[string]string{
			"X-Title":         "OpenAI Codex",
			"X-Requesty-User": "ada",
		}, provider.HTTPHeaders)

		// Codex resolves these on every launch, from what the shell hook exported.
		assert.Equal(t, map[string]string{
			"X-Requesty-Repo":   "REQUESTY_REPO",
			"X-Requesty-Branch": "REQUESTY_BRANCH",
		}, provider.EnvHTTPHeaders)
	})
}

// TestDeepSeekCarriesOnlyStaticAttribution pins that the repository and branch
// are left out rather than frozen to wherever the CLI ran, as the harness has no
// way to read a header value from the environment or a command.
func TestDeepSeekCarriesOnlyStaticAttribution(t *testing.T) {
	eachConfigurePath(t, func(t *testing.T, overwrite bool) {
		configDir := configureWithAttribution(t, func(config config.Config, dir string) Harness {
			return NewDeepSeekHarness(config, dir)
		}, overwrite)

		var settings deepseekSettings
		readYAMLFile(t, filepath.Join(configDir, "settings.yaml"), &settings)

		assert.Equal(t, map[string]string{
			"HTTP-Referer":    "https://requesty.ai",
			"X-Title":         "DeepSeek Harness",
			"X-Requesty-User": "ada",
		}, settings.PiAI.Providers[deepseekProvider].Headers)
	})
}

// TestWithoutAttributionHeadersAreUnchanged pins what a user who did not opt in
// gets: the headers that name the harness to Requesty, and nothing else.
func TestWithoutAttributionHeadersAreUnchanged(t *testing.T) {
	eachConfigurePath(t, func(t *testing.T, overwrite bool) {
		configDir := t.TempDir()
		harness := NewCodexHarness(attributionTestConfig(), configDir)

		require.NoError(t, harness.Configure(ConfigureOptions{
			Model:     attributionTestModel,
			Overwrite: overwrite,
		}))

		configPath := filepath.Join(configDir, "config.toml")

		var parsed codexConfig
		readTOMLFile(t, configPath, &parsed)
		provider := parsed.ModelProviders[codexModelProvider]
		assert.Equal(t, map[string]string{"X-Title": "OpenAI Codex"}, provider.HTTPHeaders)
		assert.Empty(t, provider.EnvHTTPHeaders)

		// An empty table would be noise in a file the user reads and edits.
		contents, err := os.ReadFile(configPath)
		require.NoError(t, err)
		assert.NotContains(t, string(contents), "env_http_headers")
	})
}

// TestAttributionKeepsHeadersAddedByHand covers the merge path only, as the
// overwrite path is the user asking for the file to be replaced.
func TestAttributionKeepsHeadersAddedByHand(t *testing.T) {
	configDir := t.TempDir()
	modelsPath := filepath.Join(configDir, "models.json")
	require.NoError(t, os.WriteFile(modelsPath, []byte(`{
		"providers": {
			"requesty": {
				"headers": {"X-Team": "platform"}
			}
		}
	}`), 0o600))

	harness := NewPiHarness(attributionTestConfig(), configDir)
	require.NoError(t, harness.Configure(ConfigureOptions{
		Model:       attributionTestModel,
		Attribution: attributionSet(),
	}))

	var models piModels
	readJSONFile(t, modelsPath, &models)
	assert.Equal(t, "platform", models.Providers[piProvider].Headers["X-Team"])
	assert.Equal(t, "!echo repo", models.Providers[piProvider].Headers["X-Requesty-Repo"])
}

// eachConfigurePath runs the test against both ways a harness writes its config,
// as the headers have to land the same way whether the file was merged into or
// replaced.
func eachConfigurePath(t *testing.T, test func(t *testing.T, overwrite bool)) {
	t.Helper()

	t.Run("merge", func(t *testing.T) { test(t, false) })
	t.Run("overwrite", func(t *testing.T) { test(t, true) })
}

// configureWithAttribution configures a harness in a directory of its own and
// returns that directory to read the written files from.
func configureWithAttribution(
	t *testing.T,
	newHarness func(config.Config, string) Harness,
	overwrite bool,
) string {
	t.Helper()

	configDir := t.TempDir()
	harness := newHarness(attributionTestConfig(), configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model:       attributionTestModel,
		Overwrite:   overwrite,
		Attribution: attributionSet(),
	}))

	return configDir
}

func attributionTestConfig() config.Config {
	return config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
}

func readJSONFile(t *testing.T, path string, into any) {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(contents, into))
}

func readTOMLFile(t *testing.T, path string, into any) {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, toml.Unmarshal(contents, into))
}

func readYAMLFile(t *testing.T, path string, into any) {
	t.Helper()

	contents, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, yaml.Unmarshal(contents, into))
}
