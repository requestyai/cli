package harnesses

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/requestyai/cli/internal/attribution"
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
	Name    string `toml:"name"`
	BaseURL string `toml:"base_url"`

	// HTTPHeaders holds fixed values, EnvHTTPHeaders the names of environment
	// variables Codex reads on every launch.
	HTTPHeaders    map[string]string `toml:"http_headers"`
	EnvHTTPHeaders map[string]string `toml:"env_http_headers,omitempty"`
}

type codexAuth struct {
	AuthMode     string `json:"auth_mode"`
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
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
		"takes a backup of current config.toml and auth.json",
		"writes a config.toml and auth.json to route through Requesty",
	}
}

func (c *CodexHarness) Status() (Status, error) {
	status := Status{}

	if _, err := exec.LookPath("codex"); err == nil {
		status.Executable = true
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	configPath := c.configPath()
	authPath := c.authPath()
	status.Files = append(status.Files, configPath, authPath)

	configExists, err := pathExists(configPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	authExists, err := pathExists(authPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	if !configExists || !authExists {
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

	if config.ModelProvider == codexModelProvider {
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
			codexModelProvider: c.provider(opts),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge config file: %w", err)
	}

	authPath := c.authPath()

	auth, err := mergeOrCreateJSONConfigFile(authPath, map[string]any{
		"auth_mode":      "apikey",
		"OPENAI_API_KEY": c.config.APIKey,
	})
	if err != nil {
		return fmt.Errorf("failed to merge auth file: %w", err)
	}

	if err := backupAndWriteConfigFileAsTOML(configPath, &config); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}
	if err := backupAndWriteConfigFileAsJSON(authPath, &auth); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}

	return nil
}

func (c *CodexHarness) configureOverwrite(opts ConfigureOptions) error {
	config := codexConfig{
		Model:         opts.Model,
		ModelProvider: codexModelProvider,
		ModelProviders: map[string]codexProvider{
			codexModelProvider: {
				Name:           "Requesty",
				BaseURL:        fmt.Sprintf("%s/v1", c.config.RouterBaseURL),
				HTTPHeaders:    c.headers(opts),
				EnvHTTPHeaders: c.envHeaders(opts),
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

	auth := codexAuth{
		AuthMode:     "apikey",
		OpenAIAPIKey: c.config.APIKey,
	}

	if err := backupAndWriteConfigFileAsJSON(c.authPath(), &auth); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}

	return nil
}

// provider describes the Requesty entry Codex routes through. The environment
// header table is left out when there is nothing to name, so a user who did not
// opt in keeps a config file without an empty table in it.
func (c *CodexHarness) provider(opts ConfigureOptions) map[string]any {
	provider := map[string]any{
		"name":         "Requesty",
		"base_url":     fmt.Sprintf("%s/v1", c.config.RouterBaseURL),
		"http_headers": headerPatch(c.headers(opts)),
	}

	if envHeaders := c.envHeaders(opts); len(envHeaders) > 0 {
		provider["env_http_headers"] = headerPatch(envHeaders)
	}

	return provider
}

// headers name Codex to Requesty and carry the attribution dimensions it can be
// given upfront.
func (c *CodexHarness) headers(opts ConfigureOptions) map[string]string {
	return opts.Attribution.Static(map[string]string{
		"X-Title": "OpenAI Codex",
	})
}

// envHeaders carry the dimensions that depend on where Codex runs. Codex reads
// them from the environment on every launch, from the variables the shell hook
// exports, which is why they are named here instead of resolved.
func (c *CodexHarness) envHeaders(opts ConfigureOptions) map[string]string {
	return opts.Attribution.Dynamic(attribution.EnvName)
}

func (c *CodexHarness) configPath() string {
	return filepath.Join(c.configDir, "config.toml")
}

func (c *CodexHarness) authPath() string {
	return filepath.Join(c.configDir, "auth.json")
}
