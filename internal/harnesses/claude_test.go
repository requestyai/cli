package harnesses

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
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

func TestClaudeHarnessLaunchInjectsRequesty(t *testing.T) {
	binaryPath := fakeBinary(t, "claude")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewClaudeHarness(cfg, t.TempDir())

	err := harness.Launch(LaunchOptions{
		Model:     "anthropic/claude-fable-5",
		FastModel: "claude-haiku-4-5",
		Effort:    EffortHigh,
		Args:      []string{"--dangerously-skip-permissions", "-p", "hello"},
		Env: []string{
			"HOME=/home/me",
			"ANTHROPIC_AUTH_TOKEN=stale-token",
			"ANTHROPIC_API_KEY=someone-elses-key",
		},
	})
	require.NoError(t, err)

	assert.Equal(t, binaryPath, captured.path)

	// Environment: Requesty replaces any Anthropic credential already present.
	assert.Contains(t, captured.env, "HOME=/home/me")
	assert.Contains(t, captured.env, "ANTHROPIC_BASE_URL=https://router.requesty.ai")
	assert.Contains(t, captured.env, "ANTHROPIC_API_KEY=my-api-key")
	assert.Contains(t, captured.env, "REQUESTY_API_KEY=my-api-key")
	assert.Contains(t, captured.env, "ANTHROPIC_MODEL=anthropic/claude-fable-5")
	assert.Contains(t, captured.env, "ANTHROPIC_DEFAULT_HAIKU_MODEL=claude-haiku-4-5")
	assert.Contains(t, captured.env, "ANTHROPIC_SMALL_FAST_MODEL=claude-haiku-4-5")
	assert.Contains(t, captured.env, "CLAUDE_CODE_SKIP_FAST_MODE_ORG_CHECK=1")
	for _, entry := range captured.env {
		assert.NotContains(t, entry, "ANTHROPIC_AUTH_TOKEN=", "the bearer token path must be cleared")
	}

	// Arguments: argv[0], then our --settings and --effort, then the user's flags verbatim.
	require.Len(t, captured.argv, 8)
	assert.Equal(t, "claude", captured.argv[0])
	assert.Equal(t, "--settings", captured.argv[1])
	assert.Equal(t, []string{"--effort", "high", "--dangerously-skip-permissions", "-p", "hello"}, captured.argv[3:])

	var settings struct {
		APIKeyHelper string            `json:"apiKeyHelper"`
		Env          map[string]string `json:"env"`
	}
	require.NoError(t, json.Unmarshal([]byte(captured.argv[2]), &settings))
	assert.Equal(t, "https://router.requesty.ai", settings.Env["ANTHROPIC_BASE_URL"])
	assert.Equal(t, "", settings.Env["CLAUDE_CODE_USE_BEDROCK"])
	assert.Equal(t, "", settings.Env["CLAUDE_CODE_USE_VERTEX"])
	assert.Equal(t, "", settings.Env["ANTHROPIC_AUTH_TOKEN"])
	assert.NotContains(t, captured.argv[2], "my-api-key", "the key stays out of argv")
	if runtime.GOOS == "windows" {
		assert.Empty(t, settings.APIKeyHelper)
	} else {
		assert.Equal(t, `printf %s "$REQUESTY_API_KEY"`, settings.APIKeyHelper)
	}
}

func TestClaudeHarnessLaunchWithoutOverridesIsMinimal(t *testing.T) {
	fakeBinary(t, "claude")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewClaudeHarness(cfg, t.TempDir())

	require.NoError(t, harness.Launch(LaunchOptions{Env: []string{}}))

	require.Len(t, captured.argv, 3, "only claude --settings <json>")
	assert.Equal(t, "--settings", captured.argv[1])
	for _, entry := range captured.env {
		assert.NotContains(t, entry, "ANTHROPIC_MODEL=", "no model means the harness keeps its default")
		assert.NotContains(t, entry, "ANTHROPIC_DEFAULT_HAIKU_MODEL=", "no fast model means the harness keeps its default")
	}
}

func TestClaudeHarnessLaunchRespectsUserSettingsFlag(t *testing.T) {
	fakeBinary(t, "claude")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewClaudeHarness(cfg, t.TempDir())

	for _, userArgs := range [][]string{
		{"--settings", "/tmp/mine.json"},
		{"--settings=/tmp/mine.json"},
		{"--setting-sources", "user"},
	} {
		require.NoError(t, harness.Launch(LaunchOptions{Args: userArgs, Env: []string{}}))
		assert.Equal(t, userArgs, captured.argv[1:], "user-managed settings must pass through untouched")
	}
}

func TestClaudeHarnessLaunchRejectsUnsupportedEffort(t *testing.T) {
	fakeBinary(t, "claude")
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewClaudeHarness(cfg, t.TempDir())

	err := harness.Launch(LaunchOptions{Effort: EffortMinimal, Env: []string{}})

	require.EqualError(t, err, `Claude Code does not support reasoning effort "minimal" (accepted: low, medium, high, xhigh, max)`)
	assert.Empty(t, captured.path, "nothing should be launched")
}

func TestClaudeHarnessLaunchRequiresBinary(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewClaudeHarness(cfg, t.TempDir())

	err := harness.Launch(LaunchOptions{Env: []string{}})

	require.EqualError(t, err, "`claude` is not on PATH; install Claude Code (https://code.claude.com/docs/en/setup) and try again")
	assert.Empty(t, captured.path)
}

func TestClaudeHarnessLaunchWrapsExecFailure(t *testing.T) {
	fakeBinary(t, "claude")
	previous := execProcess
	execProcess = func(string, []string, []string) error { return errors.New("boom") }
	t.Cleanup(func() { execProcess = previous })
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}
	harness := NewClaudeHarness(cfg, t.TempDir())

	require.EqualError(t, harness.Launch(LaunchOptions{Env: []string{}}), "failed to launch claude: boom")
}

func TestClaudeHarnessDefaultConfigDir(t *testing.T) {
	homePath, err := os.UserHomeDir()
	require.NoError(t, err)

	configDir, err := DefaultConfigDirClaudeCode()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(homePath, ".claude"), configDir)
}
