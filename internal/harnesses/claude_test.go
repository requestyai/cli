package harnesses

import (
	"os"
	"path/filepath"
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

	harness := newClaudeHarness(config, configDir)

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
	harness := newClaudeHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	}))

	settings, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"env": {
			"ANTHROPIC_BASE_URL": "https://router.requesty.ai",
			"ANTHROPIC_AUTH_TOKEN": "my-api-key",
			"ANTHROPIC_MODEL": "anthropic/claude-fable-5",
			"ANTHROPIC_CUSTOM_HEADERS": "X-Requesty-Client: requesty-cli\nX-Requesty-Client-Version: dev\nX-Title: Claude Code"
		}
	}`, string(settings))
}

func TestClaudeHarnessConfigureKeepsCustomHeaders(t *testing.T) {
	config := config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}
	configDir := t.TempDir()
	settingsPath := filepath.Join(configDir, "settings.json")
	require.NoError(t, os.WriteFile(settingsPath, []byte(`{
		"env": {"ANTHROPIC_CUSTOM_HEADERS": "X-Team: platform\nX-Requesty-Client-Version: 0.0.1\nX-Requesty-Client: requesty-cli"}
	}`), 0o600))
	harness := newClaudeHarness(config, configDir)

	require.NoError(t, harness.Configure(ConfigureOptions{
		Model: "anthropic/claude-fable-5",
	}))

	settings, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.JSONEq(t, `{
		"env": {
			"ANTHROPIC_BASE_URL": "https://router.requesty.ai",
			"ANTHROPIC_AUTH_TOKEN": "my-api-key",
			"ANTHROPIC_MODEL": "anthropic/claude-fable-5",
			"ANTHROPIC_CUSTOM_HEADERS": "X-Team: platform\nX-Requesty-Client: requesty-cli\nX-Requesty-Client-Version: dev\nX-Title: Claude Code"
		}
	}`, string(settings))
}

func TestClaudeHarnessConfigureRejectsNonStringCustomHeaders(t *testing.T) {
	configDir := t.TempDir()
	settingsPath := filepath.Join(configDir, "settings.json")
	original := []byte(`{"env": {"ANTHROPIC_CUSTOM_HEADERS": ["X-Team: platform"]}}`)
	require.NoError(t, os.WriteFile(settingsPath, original, 0o600))
	harness := newClaudeHarness(config.Config{RouterBaseURL: "https://router.requesty.ai"}, configDir)

	err := harness.Configure(ConfigureOptions{Model: "anthropic/claude-fable-5"})

	require.Error(t, err)
	settings, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	assert.Equal(t, original, settings)
}

func TestClaudeHarnessLaunchEnvAddsAttributionHeaders(t *testing.T) {
	harness := newClaudeHarness(config.Config{
		RouterBaseURL: "https://router.requesty.ai",
		APIKey:        "my-api-key",
	}, t.TempDir())

	env := harness.launchEnv(LaunchOptions{
		Env: []string{"ANTHROPIC_CUSTOM_HEADERS=X-Team: platform"},
	})

	assert.Equal(t, "X-Team: platform\nX-Requesty-Client: requesty-cli\nX-Requesty-Client-Version: dev\nX-Title: Claude Code", env["ANTHROPIC_CUSTOM_HEADERS"])
}

func TestWithHeaderLines(t *testing.T) {
	headers := map[string]string{"X-Title": "Tool", "X-Requesty-Client": "requesty-cli"}
	const ourLines = "X-Requesty-Client: requesty-cli\nX-Title: Tool"

	tests := []struct {
		name     string
		existing string
		want     string
	}{
		{
			name:     "empty",
			existing: "",
			want:     ourLines,
		},
		{
			name:     "keeps the user's headers",
			existing: "X-Team: platform\nX-Env: prod",
			want:     "X-Team: platform\nX-Env: prod\n" + ourLines,
		},
		{
			name:     "replaces earlier values of our headers whatever their case",
			existing: "x-requesty-client: other\nX-TITLE: Old\nX-Team: platform",
			want:     "X-Team: platform\n" + ourLines,
		},
		{
			name:     "drops carriage returns and blank lines",
			existing: "X-Team: platform\r\n\r\nX-Env: prod\r\n",
			want:     "X-Team: platform\nX-Env: prod\n" + ourLines,
		},
		{
			name:     "keeps values containing colons",
			existing: "X-Callback: https://example.com:8443/hook",
			want:     "X-Callback: https://example.com:8443/hook\n" + ourLines,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, withHeaderLines(tt.existing, headers))
		})
	}
}
