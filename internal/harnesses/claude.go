package harnesses

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	config config.Config
}

func NewClaudeHarness(config config.Config) *ClaudeHarness {
	return &ClaudeHarness{
		config: config,
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

	if _, err := exec.LookPath("claude"); err == nil {
		status.Executable = true
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	settingsPath, err := c.settingsPath()
	if err != nil {
		return status, fmt.Errorf("failed to get settings path: %w", err)
	}
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
	settingsPath, err := c.settingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
	}

	settings, err := mergeJSONConfigFile(settingsPath, map[string]any{
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

	settingsPath, err := c.settingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
	}

	if err := backupAndWriteConfigFileAsJSON(settingsPath, &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func (c *ClaudeHarness) settingsPath() (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home path: %w", err)
	}

	return filepath.Join(homePath, ".claude", "settings.json"), nil
}
