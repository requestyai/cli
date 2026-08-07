package harnesses

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMergeJSONConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	require.NoError(t, os.WriteFile(path, []byte(`{
		"theme": "dark",
		"large_number": 9007199254740993,
		"env": {
			"EXISTING_VARIABLE": "keep-me",
			"ANTHROPIC_MODEL": "old-model"
		},
		"permissions": {
			"allow": ["Bash"]
		}
	}`), 0o600))

	merged, err := mergeJSONConfigFile(path, map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":   "https://router.requesty.ai",
			"ANTHROPIC_AUTH_TOKEN": "my-api-key",
			"ANTHROPIC_MODEL":      "anthropic/claude-fable-5",
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"theme":        "dark",
		"large_number": json.Number("9007199254740993"),
		"env": map[string]any{
			"EXISTING_VARIABLE":    "keep-me",
			"ANTHROPIC_BASE_URL":   "https://router.requesty.ai",
			"ANTHROPIC_AUTH_TOKEN": "my-api-key",
			"ANTHROPIC_MODEL":      "anthropic/claude-fable-5",
		},
		"permissions": map[string]any{
			"allow": []any{"Bash"},
		},
	}, merged)
}

func TestMergeTOMLConfigFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	require.NoError(t, os.WriteFile(path, []byte(`
custom_setting = "keep-me"
model = "old-model"

[model_providers.other]
name = "Other provider"

[model_providers.requesty]
custom_provider_setting = "keep-me"

[model_providers.requesty.http_headers]
Existing = "keep-me"

[projects."/existing/project"]
trust_level = "untrusted"

[projects."/home/test-user"]
custom_project_setting = "keep-me"
`), 0o600))

	merged, err := mergeTOMLConfigFile(path, map[string]any{
		"model":                              "openai-responses/gpt-5.5",
		"model_provider":                     codexModelProvider,
		"model_supports_reasoning_summaries": false,
		"model_providers": map[string]any{
			codexModelProvider: map[string]any{
				"base_url": "https://router.requesty.ai/v1",
				"http_headers": map[string]any{
					"X-Title": "OpenAI Codex",
				},
			},
		},
	})
	require.NoError(t, err)
	require.Equal(t, map[string]any{
		"custom_setting":                     "keep-me",
		"model":                              "openai-responses/gpt-5.5",
		"model_provider":                     codexModelProvider,
		"model_supports_reasoning_summaries": false,
		"model_providers": map[string]any{
			"other": map[string]any{
				"name": "Other provider",
			},
			codexModelProvider: map[string]any{
				"custom_provider_setting": "keep-me",
				"base_url":                "https://router.requesty.ai/v1",
				"http_headers": map[string]any{
					"Existing": "keep-me",
					"X-Title":  "OpenAI Codex",
				},
			},
		},
		"projects": map[string]any{
			"/existing/project": map[string]any{
				"trust_level": "untrusted",
			},
			"/home/test-user": map[string]any{
				"custom_project_setting": "keep-me",
			},
		},
	}, merged)
}
