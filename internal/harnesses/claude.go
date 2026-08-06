package harnesses

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type claudeSettings struct {
	Env struct {
		AnthropicBaseURL   string `json:"ANTHROPIC_BASE_URL"`
		AnthropicAuthToken string `json:"ANTHROPIC_AUTH_TOKEN"`
		AnthropicModel     string `json:"ANTHROPIC_MODEL"`
	} `json:"env"`
}

type ClaudeHarness struct{}

func NewClaudeHarness() *ClaudeHarness {
	return &ClaudeHarness{}
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

	if settings.Env.AnthropicBaseURL == "https://router.requesty.ai" {
		status.Configured = true
	} else {
		status.Configured = false
	}

	return status, nil
}

func (c *ClaudeHarness) Configure(opts ConfigureOptions) error {
	settingsPath, err := c.settingsPath()
	if err != nil {
		return fmt.Errorf("failed to get settings path: %w", err)
	}

	if err := backupFile(settingsPath); err != nil {
		return fmt.Errorf("failed to backup file: %w", err)
	}

	settings := claudeSettings{}
	settings.Env.AnthropicBaseURL = "https://router.requesty.ai"
	settings.Env.AnthropicAuthToken = opts.APIKey
	settings.Env.AnthropicModel = opts.Model

	settingsBytes, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	if err := writeFile(settingsPath, settingsBytes, 0o600); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
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
