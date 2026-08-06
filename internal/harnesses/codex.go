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
	Projects       map[string]codexProject  `toml:"projects"`
}

type codexProvider struct {
	Name        string            `toml:"name"`
	BaseURL     string            `toml:"base_url"`
	HTTPHeaders map[string]string `toml:"http_headers"`
}

type codexProject struct {
	TrustLevel string `toml:"trust_level"`
}

type codexAuth struct {
	AuthMode     string `json:"auth_mode"`
	OpenAIAPIKey string `json:"OPENAI_API_KEY"`
}

type CodexHarness struct {
	config config.Config
}

func NewCodexHarness(config config.Config) *CodexHarness {
	return &CodexHarness{
		config: config,
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

	configPath, err := c.configPath()
	if err != nil {
		return status, fmt.Errorf("failed to get config path: %w", err)
	}
	status.Files = append(status.Files, configPath)

	authPath, err := c.authPath()
	if err != nil {
		return status, fmt.Errorf("failed to get auth path: %w", err)
	}
	status.Files = append(status.Files, authPath)

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
	homePath, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home path: %w", err)
	}

	config := codexConfig{
		Model:         opts.Model,
		ModelProvider: codexModelProvider,
		ModelProviders: map[string]codexProvider{
			codexModelProvider: {
				Name:    "Requesty",
				BaseURL: fmt.Sprintf("%s/v1", c.config.BaseURL),
				HTTPHeaders: map[string]string{
					"X-Title": "OpenAI Codex",
				},
			},
		},
		ModelReasoningEffort:            "high",
		ModelSupportsReasoningSummaries: false,
		WebSearch:                       "live",
		Personality:                     "pragmatic",
		Projects: map[string]codexProject{
			homePath: {
				TrustLevel: "trusted",
			},
		},
	}

	configPath, err := c.configPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %w", err)
	}

	if err := backupAndWriteConfigFileAsTOML(configPath, &config); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	auth := codexAuth{
		AuthMode:     "apikey",
		OpenAIAPIKey: c.config.APIKey,
	}

	authPath, err := c.authPath()
	if err != nil {
		return fmt.Errorf("failed to get auth path: %w", err)
	}

	if err := backupAndWriteConfigFileAsJSON(authPath, &auth); err != nil {
		return fmt.Errorf("failed to write auth file: %w", err)
	}

	return nil
}

func (c *CodexHarness) configPath() (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home path: %w", err)
	}

	return filepath.Join(homePath, ".codex", "config.toml"), nil
}

func (c *CodexHarness) authPath() (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home path: %w", err)
	}

	return filepath.Join(homePath, ".codex", "auth.json"), nil
}
