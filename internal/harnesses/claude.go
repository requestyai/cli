package harnesses

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/requestyai/cli/internal/config"
)

const claudeTitle = "Claude Code"

type claudeSettings struct {
	Env struct {
		AnthropicBaseURL       string `json:"ANTHROPIC_BASE_URL"`
		AnthropicAuthToken     string `json:"ANTHROPIC_AUTH_TOKEN"`
		AnthropicModel         string `json:"ANTHROPIC_MODEL"`
		AnthropicCustomHeaders string `json:"ANTHROPIC_CUSTOM_HEADERS,omitempty"`
	} `json:"env"`
}

type ClaudeHarness struct {
	config    config.Config
	configDir string
}

// NewClaudeHarness is Claude Code at its default configuration directory.
func NewClaudeHarness(config config.Config) (Harness, error) {
	configDir, err := configDirInHome(".claude")
	if err != nil {
		return nil, err
	}

	return newClaudeHarness(config, configDir), nil
}

func newClaudeHarness(config config.Config, configDir string) *ClaudeHarness {
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

	status.Files = append(status.Files, c.settingsPath())

	settings, err := c.readSettings()
	if err != nil {
		return status, err
	}
	status.Configured = settings.Env.AnthropicBaseURL == c.config.RouterBaseURL

	return status, nil
}

// readSettings is the user's settings.json, or empty settings when there is
// none yet.
func (c *ClaudeHarness) readSettings() (claudeSettings, error) {
	var settings claudeSettings

	data, err := os.ReadFile(c.settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	} else if err != nil {
		return settings, fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(data, &settings); err != nil {
		return settings, fmt.Errorf("failed to unmarshal: %w", err)
	}

	return settings, nil
}

func (c *ClaudeHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return c.configureOverwrite(opts)
	}

	return c.configureMerge(opts)
}

func (c *ClaudeHarness) configureMerge(opts ConfigureOptions) error {
	settingsPath := c.settingsPath()

	existing, err := c.readSettings()
	if err != nil {
		return fmt.Errorf("failed to read settings file: %w", err)
	}

	settings, err := mergeOrCreateJSONConfigFile(settingsPath, map[string]any{
		"env": map[string]any{
			"ANTHROPIC_BASE_URL":       c.config.RouterBaseURL,
			"ANTHROPIC_AUTH_TOKEN":     c.config.APIKey,
			"ANTHROPIC_MODEL":          opts.Model,
			"ANTHROPIC_CUSTOM_HEADERS": withHeaderLines(existing.Env.AnthropicCustomHeaders, attributionHeaders(claudeTitle)),
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
	settings.Env.AnthropicCustomHeaders = withHeaderLines("", attributionHeaders(claudeTitle))

	if err := backupAndWriteConfigFileAsJSON(c.settingsPath(), &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func (c *ClaudeHarness) settingsPath() string {
	return filepath.Join(c.configDir, "settings.json")
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

	env := c.launchEnv(opts)

	argv := []string{"claude"}
	if !hasAnyFlag(opts.Args, claudeSettingsFlags) {
		settings, err := c.inlineSettings()
		if err != nil {
			return err
		}
		argv = append(argv, "--settings", settings)
	}
	argv = append(argv, opts.Args...)

	if err := execProcess(status.ExecutablePath, argv, formatEnvironmentVariables(env)); err != nil {
		return fmt.Errorf("failed to launch claude: %w", err)
	}

	return nil
}

func (c *ClaudeHarness) launchEnv(opts LaunchOptions) map[string]string {
	env := parseEnvironmentVariables(opts.Env)
	env["ANTHROPIC_BASE_URL"] = c.config.RouterBaseURL
	env["ANTHROPIC_API_KEY"] = c.config.APIKey
	env["REQUESTY_API_KEY"] = c.config.APIKey
	env["ANTHROPIC_CUSTOM_HEADERS"] = withHeaderLines(env["ANTHROPIC_CUSTOM_HEADERS"], attributionHeaders(claudeTitle))
	delete(env, "ANTHROPIC_AUTH_TOKEN")
	// The fast-mode check calls an Anthropic-only org endpoint that a gateway
	// cannot answer; skipping it avoids a startup warning.
	env["CLAUDE_CODE_SKIP_FAST_MODE_ORG_CHECK"] = "1"
	if opts.Model != "" {
		env["ANTHROPIC_MODEL"] = opts.Model
	}
	if opts.FastModel != "" {
		// The haiku alias covers background work (titles, summaries,
		// subagents declared as haiku). ANTHROPIC_SMALL_FAST_MODEL is the
		// name versions before 2.1.2 read; both get the same value.
		env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] = opts.FastModel
		env["ANTHROPIC_SMALL_FAST_MODEL"] = opts.FastModel
	}

	return env
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

// withHeaderLines adds headers to existing, a newline-separated list of
// `Name: Value` lines as ANTHROPIC_CUSTOM_HEADERS expects. Earlier lines for
// the same names are replaced; the user's other headers are kept.
func withHeaderLines(existing string, headers map[string]string) string {
	replaced := func(name string) bool {
		for header := range headers {
			if strings.EqualFold(name, header) {
				return true
			}
		}
		return false
	}

	lines := []string{}
	for _, line := range strings.Split(existing, "\n") {
		line = strings.TrimSuffix(line, "\r")
		name, _, _ := strings.Cut(line, ":")
		name = strings.TrimSpace(name)
		if name == "" || replaced(name) {
			continue
		}
		lines = append(lines, line)
	}
	for _, name := range slices.Sorted(maps.Keys(headers)) {
		lines = append(lines, name+": "+headers[name])
	}

	return strings.Join(lines, "\n")
}
