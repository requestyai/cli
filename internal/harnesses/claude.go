package harnesses

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/requestyai/cli/internal/config"
)

type claudeSettings struct {
	Env struct {
		AnthropicBaseURL   string `json:"ANTHROPIC_BASE_URL"`
		AnthropicAuthToken string `json:"ANTHROPIC_AUTH_TOKEN"`
		AnthropicModel     string `json:"ANTHROPIC_MODEL"`
	} `json:"env"`
}

type ClaudeHarness struct {
	config    config.Config
	configDir string
}

// DefaultConfigDirClaudeCode is where Claude Code keeps its configuration.
func DefaultConfigDirClaudeCode() (string, error) {
	return configDirInHome(".claude")
}

func NewClaudeHarness(config config.Config, configDir string) *ClaudeHarness {
	return &ClaudeHarness{
		config:    config,
		configDir: configDir,
	}
}

func (c *ClaudeHarness) Name() string {
	return "Claude Code"
}

func (c *ClaudeHarness) Description() []string {
	return []string{
		"takes a backup of settings.json",
		"writes a settings.json to route through Requesty",
	}
}

func (c *ClaudeHarness) Status() (Status, error) {
	status := Status{}

	if path, err := exec.LookPath("claude"); err == nil {
		status.Executable = true
		status.ExecutablePath = path
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	settingsPath := c.settingsPath()
	status.Files = append(status.Files, settingsPath)

	settingsExists, err := pathExists(settingsPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	if !settingsExists {
		status.Configured = false
		return status, nil
	}

	settingsBytes, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		status.Configured = false
	} else if err != nil {
		return status, fmt.Errorf("failed to read file: %w", err)
	}

	var settings claudeSettings
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		return status, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if settings.Env.AnthropicBaseURL == c.config.RouterBaseURL {
		status.Configured = true
	} else {
		status.Configured = false
	}

	return status, nil
}

func (c *ClaudeHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return c.configureOverwrite(opts)
	}

	return c.configureMerge(opts)
}

func (c *ClaudeHarness) configureMerge(opts ConfigureOptions) error {
	settingsPath := c.settingsPath()

	settings, err := mergeOrCreateJSONConfigFile(settingsPath, map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":   c.config.RouterBaseURL,
			"ANTHROPIC_AUTH_TOKEN": c.config.APIKey,
			"ANTHROPIC_MODEL":      opts.Model,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge settings file: %w", err)
	}

	if err := backupAndWriteConfigFileAsJSON(settingsPath, &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func (c *ClaudeHarness) configureOverwrite(opts ConfigureOptions) error {
	settings := claudeSettings{}
	settings.Env.AnthropicBaseURL = c.config.RouterBaseURL
	settings.Env.AnthropicAuthToken = c.config.APIKey
	settings.Env.AnthropicModel = opts.Model

	if err := backupAndWriteConfigFileAsJSON(c.settingsPath(), &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func (c *ClaudeHarness) settingsPath() string {
	return filepath.Join(c.configDir, "settings.json")
}

// claudeEfforts maps Launch effort levels onto Claude Code's --effort values.
var claudeEfforts = map[string]string{
	EffortLow:    "low",
	EffortMedium: "medium",
	EffortHigh:   "high",
	EffortXHigh:  "xhigh",
	EffortMax:    "max",
}

// claudeOverrideSettings are passed inline with --settings so they outrank
// the user's own settings.json for this run only. Every alternative-provider
// switch is blanked so a Bedrock or Vertex setup on the machine cannot route
// around Requesty; the credential itself stays out of argv and travels in
// the environment.
var claudeOverrideSettings = map[string]string{
	"ANTHROPIC_AUTH_TOKEN":                   "",
	"ANTHROPIC_AWS_BASE_URL":                 "",
	"ANTHROPIC_BEDROCK_BASE_URL":             "",
	"ANTHROPIC_BEDROCK_MANTLE_BASE_URL":      "",
	"ANTHROPIC_FOUNDRY_BASE_URL":             "",
	"ANTHROPIC_GOOGLE_CLOUD_BASE_URL":        "",
	"ANTHROPIC_UNIX_SOCKET":                  "",
	"ANTHROPIC_VERTEX_BASE_URL":              "",
	"CLAUDE_CODE_OAUTH_TOKEN":                "",
	"CLAUDE_CODE_USE_ANTHROPIC_AWS":          "",
	"CLAUDE_CODE_USE_ANTHROPIC_GOOGLE_CLOUD": "",
	"CLAUDE_CODE_USE_BEDROCK":                "",
	"CLAUDE_CODE_USE_FOUNDRY":                "",
	"CLAUDE_CODE_USE_GATEWAY":                "",
	"CLAUDE_CODE_USE_MANTLE":                 "",
	"CLAUDE_CODE_USE_VERTEX":                 "",
}

// claudeSettingsFlags are Claude Code flags that mean the user is managing
// settings themselves, in which case we do not add our own --settings.
var claudeSettingsFlags = []string{"--settings", "--setting-sources"}

// Launch replaces this process with Claude Code pointed at Requesty. The base
// URL and key go in the environment; an inline --settings document pins the
// base URL above the user's settings.json and disables every other provider
// path. Nothing is written to disk.
func (c *ClaudeHarness) Launch(opts LaunchOptions) error {
	status, err := c.Status()
	if err != nil {
		return fmt.Errorf("failed to check %s: %w", c.Name(), err)
	}
	if !status.Executable {
		return fmt.Errorf("`claude` is not on PATH; install Claude Code (https://code.claude.com/docs/en/setup) and try again")
	}

	effort, err := mapEffort(c.Name(), claudeEfforts, opts.Effort)
	if err != nil {
		return err
	}

	env := parseEnvironmentVariables(opts.Env)
	env["ANTHROPIC_BASE_URL"] = c.config.RouterBaseURL
	env["ANTHROPIC_API_KEY"] = c.config.APIKey
	env["REQUESTY_API_KEY"] = c.config.APIKey
	delete(env, "ANTHROPIC_AUTH_TOKEN")
	// The fast-mode check calls an Anthropic-only org endpoint that a gateway
	// cannot answer; skipping it avoids a startup warning.
	env["CLAUDE_CODE_SKIP_FAST_MODE_ORG_CHECK"] = "1"
	if opts.Model != "" {
		env["ANTHROPIC_MODEL"] = opts.Model
	}

	argv := []string{"claude"}
	if !hasAnyFlag(opts.Args, claudeSettingsFlags) {
		settings, err := c.inlineSettings()
		if err != nil {
			return err
		}
		argv = append(argv, "--settings", settings)
	}
	if effort != "" {
		argv = append(argv, "--effort", effort)
	}
	argv = append(argv, opts.Args...)

	if err := execProcess(status.ExecutablePath, argv, formatEnvironmentVariables(env)); err != nil {
		return fmt.Errorf("failed to launch claude: %w", err)
	}

	return nil
}

func (c *ClaudeHarness) inlineSettings() (string, error) {
	settingsEnv := make(map[string]string, len(claudeOverrideSettings)+1)
	for key, value := range claudeOverrideSettings {
		settingsEnv[key] = value
	}
	settingsEnv["ANTHROPIC_BASE_URL"] = c.config.RouterBaseURL

	settings := map[string]any{"env": settingsEnv}
	if runtime.GOOS != "windows" {
		// Claude Code runs the helper through a shell, which Windows lacks a
		// reliable one for; there the ANTHROPIC_API_KEY variable carries it.
		settings["apiKeyHelper"] = `printf %s "$REQUESTY_API_KEY"`
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("failed to encode inline settings: %w", err)
	}

	return string(data), nil
}

// hasAnyFlag reports whether args names one of flags, as `--flag` or
// `--flag=value`.
func hasAnyFlag(args []string, flags []string) bool {
	for _, arg := range args {
		for _, flag := range flags {
			if arg == flag || strings.HasPrefix(arg, flag+"=") {
				return true
			}
		}
	}

	return false
}
