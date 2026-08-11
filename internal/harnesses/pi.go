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

const (
	piProvider             = "requesty"
	piDefaultThinkingLevel = "medium"

	// piAPI is the native Anthropic Messages format, which lets Requesty apply
	// automatic prompt caching. It requires a base URL without the /v1 suffix.
	piAPI = "anthropic-messages"
)

type piModels struct {
	Providers map[string]piProviderConfig `json:"providers"`
}

type piSettings struct {
	DefaultProvider      string `json:"defaultProvider"`
	DefaultModel         string `json:"defaultModel"`
	DefaultThinkingLevel string `json:"defaultThinkingLevel"`
}

type piProviderConfig struct {
	Name    string            `json:"name"`
	BaseURL string            `json:"baseUrl"`
	API     string            `json:"api"`
	APIKey  string            `json:"apiKey"`
	Headers map[string]string `json:"headers,omitempty"`
	Models  []piModelConfig   `json:"models"`
}

type piModelConfig struct {
	ID string `json:"id"`
}

type PiHarness struct {
	config    config.Config
	configDir string
}

// DefaultConfigDirPi is where Pi keeps its agent configuration.
func DefaultConfigDirPi() (string, error) {
	return configDirInHome(".pi", "agent")
}

func NewPiHarness(config config.Config, configDir string) *PiHarness {
	return &PiHarness{
		config:    config,
		configDir: configDir,
	}
}

func (p *PiHarness) Name() string {
	return "Pi"
}

func (p *PiHarness) Description() []string {
	return []string{
		"takes a backup of models.json and settings.json",
		"writes models.json and settings.json to route through Requesty",
	}
}

func (p *PiHarness) Status() (Status, error) {
	status := Status{}

	if _, err := exec.LookPath("pi"); err == nil {
		status.Executable = true
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	modelsPath := p.modelsPath()
	settingsPath := p.settingsPath()
	status.Files = append(status.Files, modelsPath, settingsPath)

	modelsExists, err := pathExists(modelsPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	settingsExists, err := pathExists(settingsPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	if !modelsExists || !settingsExists {
		status.Configured = false
		return status, nil
	}

	modelsBytes, err := os.ReadFile(modelsPath)
	if err != nil {
		return status, fmt.Errorf("failed to read file: %w", err)
	}

	var models piModels
	if err := json.Unmarshal(modelsBytes, &models); err != nil {
		return status, fmt.Errorf("failed to unmarshal: %w", err)
	}

	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		return status, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings piSettings
	if err := json.Unmarshal(settingsBytes, &settings); err != nil {
		return status, fmt.Errorf("failed to unmarshal settings: %w", err)
	}

	status.Configured = models.Providers[piProvider].BaseURL == p.config.RouterBaseURL &&
		settings.DefaultProvider == piProvider

	return status, nil
}

func (p *PiHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return p.configureOverwrite(opts)
	}

	return p.configureMerge(opts)
}

func (p *PiHarness) configureMerge(opts ConfigureOptions) error {
	modelsPath := p.modelsPath()

	models, err := mergeOrCreateJSONConfigFile(modelsPath, map[string]any{
		"providers": map[string]any{
			piProvider: map[string]any{
				"name":    "Requesty",
				"baseUrl": p.config.RouterBaseURL,
				"api":     piAPI,
				"apiKey":  p.config.APIKey,
				"headers": map[string]any{
					"HTTP-Referer": "https://pi.dev",
					"X-Title":      "Pi",
				},
				"models": []any{
					map[string]any{"id": opts.Model},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge models file: %w", err)
	}

	if err := backupAndWriteConfigFileAsJSON(modelsPath, &models); err != nil {
		return fmt.Errorf("failed to write models file: %w", err)
	}

	settingsPath := p.settingsPath()
	settings, err := mergeOrCreateJSONConfigFile(settingsPath, map[string]any{
		"defaultProvider":      piProvider,
		"defaultModel":         opts.Model,
		"defaultThinkingLevel": piDefaultThinkingLevel,
	})
	if err != nil {
		return fmt.Errorf("failed to merge settings file: %w", err)
	}

	if err := backupAndWriteConfigFileAsJSON(settingsPath, &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func (p *PiHarness) configureOverwrite(opts ConfigureOptions) error {
	models := piModels{
		Providers: map[string]piProviderConfig{
			piProvider: {
				Name:    "Requesty",
				BaseURL: p.config.RouterBaseURL,
				API:     piAPI,
				APIKey:  p.config.APIKey,
				Headers: map[string]string{
					"HTTP-Referer": "https://pi.dev",
					"X-Title":      "Pi",
				},
				Models: []piModelConfig{
					{ID: opts.Model},
				},
			},
		},
	}

	if err := backupAndWriteConfigFileAsJSON(p.modelsPath(), &models); err != nil {
		return fmt.Errorf("failed to write models file: %w", err)
	}

	settings := piSettings{
		DefaultProvider:      piProvider,
		DefaultModel:         opts.Model,
		DefaultThinkingLevel: piDefaultThinkingLevel,
	}

	if err := backupAndWriteConfigFileAsJSON(p.settingsPath(), &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	return nil
}

func (p *PiHarness) modelsPath() string {
	return filepath.Join(p.configDir, "models.json")
}

func (p *PiHarness) settingsPath() string {
	return filepath.Join(p.configDir, "settings.json")
}
