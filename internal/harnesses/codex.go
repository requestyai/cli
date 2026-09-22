package harnesses

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"github.com/requestyai/cli/internal/config"
)

const (
	codexModelProvider = "requesty"
)

type codexConfig struct {
	Model                           string `toml:"model"`
	ModelProvider                   string `toml:"model_provider"`
	ModelReasoningEffort            string `toml:"model_reasoning_effort"`
	ModelSupportsReasoningSummaries bool   `toml:"model_supports_reasoning_summaries"`
	WebSearch                       string `toml:"web_search"`
	Personality                     string `toml:"personality"`

	ModelProviders map[string]codexProvider `toml:"model_providers"`
}

type codexProvider struct {
	Name        string            `toml:"name"`
	BaseURL     string            `toml:"base_url"`
	HTTPHeaders map[string]string `toml:"http_headers"`
	Auth        codexProviderAuth `toml:"auth"`
}

type codexProviderAuth struct {
	Command string   `toml:"command"`
	Args    []string `toml:"args"`
}

type CodexHarness struct {
	config    config.Config
	configDir string
}

// DefaultConfigDirCodex is where Codex keeps its configuration.
func DefaultConfigDirCodex() (string, error) {
	return configDirInHome(".codex")
}

func NewCodexHarness(config config.Config, configDir string) *CodexHarness {
	return &CodexHarness{
		config:    config,
		configDir: configDir,
	}
}

func (c *CodexHarness) Name() string {
	return "Codex"
}

func (c *CodexHarness) Description() []string {
	return []string{
		"takes a backup of current config.toml",
		"writes a config.toml to route through Requesty",
	}
}

func (c *CodexHarness) Status() (Status, error) {
	status := Status{}

	if path, err := exec.LookPath("codex"); err == nil {
		status.Executable = true
		status.ExecutablePath = path
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	configPath := c.configPath()
	status.Files = append(status.Files, configPath)

	configExists, err := pathExists(configPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	if !configExists {
		status.Configured = false
		return status, nil
	}

	configBytes, err := os.ReadFile(configPath)
	if errors.Is(err, os.ErrNotExist) {
		status.Configured = false
	} else if err != nil {
		return status, fmt.Errorf("failed to read file: %w", err)
	}

	var config codexConfig
	if err := toml.Unmarshal(configBytes, &config); err != nil {
		return status, fmt.Errorf("failed to unmarshal: %w", err)
	}

	provider, providerExists := config.ModelProviders[codexModelProvider]
	if config.ModelProvider == codexModelProvider && providerExists && provider.Auth.Command != "" {
		status.Configured = true
	} else {
		status.Configured = false
	}

	return status, nil
}

func (c *CodexHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return c.configureOverwrite(opts)
	}

	return c.configureMerge(opts)
}

func (c *CodexHarness) configureMerge(opts ConfigureOptions) error {
	configPath := c.configPath()

	config, err := mergeOrCreateTOMLConfigFile(configPath, map[string]any{
		"model":                              opts.Model,
		"model_provider":                     codexModelProvider,
		"model_reasoning_effort":             "high",
		"model_supports_reasoning_summaries": false,
		"web_search":                         "live",
		"personality":                        "pragmatic",
		"model_providers": map[string]any{
			codexModelProvider: map[string]any{
				"name":     "Requesty",
				"base_url": fmt.Sprintf("%s/v1", c.config.RouterBaseURL),
				"http_headers": map[string]any{
					"X-Title": "OpenAI Codex",
				},
				"auth": map[string]any{
					"command": "requesty",
					"args":    c.authArgs(),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge config file: %w", err)
	}

	if err := backupAndWriteConfigFileAsTOML(configPath, &config); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *CodexHarness) configureOverwrite(opts ConfigureOptions) error {
	config := codexConfig{
		Model:         opts.Model,
		ModelProvider: codexModelProvider,
		ModelProviders: map[string]codexProvider{
			codexModelProvider: {
				Name:    "Requesty",
				BaseURL: fmt.Sprintf("%s/v1", c.config.RouterBaseURL),
				HTTPHeaders: map[string]string{
					"X-Title": "OpenAI Codex",
				},
				Auth: codexProviderAuth{
					Command: "requesty",
					Args:    c.authArgs(),
				},
			},
		},
		ModelReasoningEffort:            "high",
		ModelSupportsReasoningSummaries: false,
		WebSearch:                       "live",
		Personality:                     "pragmatic",
	}

	if err := backupAndWriteConfigFileAsTOML(c.configPath(), &config); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (c *CodexHarness) configPath() string {
	return filepath.Join(c.configDir, "config.toml")
}

func (c *CodexHarness) authArgs() []string {
	return []string{"auth", "token", "--profile", c.config.Name}
}

// codexEfforts maps Launch effort levels onto Codex's model_reasoning_effort.
var codexEfforts = map[string]string{
	EffortMinimal: "minimal",
	EffortLow:     "low",
	EffortMedium:  "medium",
	EffortHigh:    "high",
	EffortXHigh:   "xhigh",
}

// Launch replaces this process with Codex pointed at Requesty using
// `-c key=value` overrides, which outrank ~/.codex/config.toml for this run
// only. Nothing is written to disk; the key is fetched on demand through
// `requesty auth token`, so it appears in neither argv nor the environment.
func (c *CodexHarness) Launch(opts LaunchOptions) error {
	status, err := c.Status()
	if err != nil {
		return fmt.Errorf("failed to check %s: %w", c.Name(), err)
	}
	if !status.Executable {
		return fmt.Errorf("`codex` is not on PATH; install Codex (https://developers.openai.com/codex/cli) and try again")
	}

	effort, err := mapEffort(c.Name(), codexEfforts, opts.Effort)
	if err != nil {
		return err
	}

	provider := "model_providers." + codexModelProvider
	argv := []string{
		"codex",
		"-c", "model_provider=" + tomlString(codexModelProvider),
		"-c", provider + ".name=" + tomlString("Requesty"),
		"-c", provider + ".base_url=" + tomlString(c.config.RouterBaseURL+"/v1"),
		"-c", provider + ".http_headers.X-Title=" + tomlString("OpenAI Codex"),
		"-c", provider + ".auth.command=" + tomlString(requestyExecutable()),
		"-c", provider + ".auth.args=[" + tomlString("auth") + "," + tomlString("token") + "," + tomlString("--profile") + "," + tomlString(c.config.Name) + "]",
		"-c", "model_supports_reasoning_summaries=false",
	}
	if opts.Model != "" {
		argv = append(argv, "-m", opts.Model)
	}
	if effort != "" {
		argv = append(argv, "-c", "model_reasoning_effort="+tomlString(effort))
	}
	argv = append(argv, opts.Args...)

	env := opts.Env
	if env == nil {
		env = os.Environ()
	}

	if err := execProcess(status.ExecutablePath, argv, env); err != nil {
		return fmt.Errorf("failed to launch codex: %w", err)
	}

	return nil
}

// tomlString quotes s as a TOML string for a Codex -c override. Codex parses
// the value side as TOML, so unquoted text with spaces or slashes would be
// rejected. Literal strings are preferred because they need no escaping.
func tomlString(s string) string {
	if !strings.ContainsAny(s, "'\n\r") {
		return "'" + s + "'"
	}

	return strconv.Quote(s)
}
