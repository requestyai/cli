package harnesses

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

	if _, err := exec.LookPath("codex"); err == nil {
		status.Executable = true
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
					"args":    []string{"auth", "token"},
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
					Args:    []string{"auth", "token"},
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
