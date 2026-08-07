package harnesses

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/requestyai/cli/internal/config"
	"gopkg.in/yaml.v3"
)

const (
	hermesProvider = "requesty"

	// hermesAPIMode is the native Anthropic Messages format, which lets Requesty
	// apply automatic prompt caching to Hermes' large system prompt.
	hermesAPIMode = "anthropic_messages"
)

type hermesConfig struct {
	Model           hermesModel            `yaml:"model"`
	CustomProviders []hermesCustomProvider `yaml:"custom_providers"`
}

type hermesModel struct {
	Default        string            `yaml:"default"`
	Provider       string            `yaml:"provider"`
	DefaultHeaders map[string]string `yaml:"default_headers,omitempty"`
}

type hermesCustomProvider struct {
	Name    string `yaml:"name"`
	BaseURL string `yaml:"base_url"`
	APIKey  string `yaml:"api_key"`
	APIMode string `yaml:"api_mode"`
}

type HermesHarness struct {
	config    config.Config
	configDir string
}

// DefaultConfigDirHermes is where Hermes keeps its configuration.
func DefaultConfigDirHermes() (string, error) {
	return configDirInHome(".hermes")
}

func NewHermesHarness(config config.Config, configDir string) *HermesHarness {
	return &HermesHarness{
		config:    config,
		configDir: configDir,
	}
}

func (h *HermesHarness) Name() string {
	return "Hermes"
}

func (h *HermesHarness) Description() []string {
	return []string{
		"takes a backup of config.yaml",
		"writes a config.yaml to route through Requesty",
	}
}

func (h *HermesHarness) Status() (Status, error) {
	status := Status{}

	if _, err := exec.LookPath("hermes"); err == nil {
		status.Executable = true
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	configPath := h.configPath()
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
	if err != nil {
		return status, fmt.Errorf("failed to read file: %w", err)
	}

	var settings hermesConfig
	if err := yaml.Unmarshal(configBytes, &settings); err != nil {
		return status, fmt.Errorf("failed to unmarshal: %w", err)
	}

	if settings.Model.Provider != hermesProvider {
		status.Configured = false
		return status, nil
	}

	for _, provider := range settings.CustomProviders {
		if provider.Name == hermesProvider && provider.BaseURL == h.config.RouterBaseURL {
			status.Configured = true
			break
		}
	}

	return status, nil
}

func (h *HermesHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return h.configureOverwrite(opts)
	}

	return h.configureMerge(opts)
}

func (h *HermesHarness) configureMerge(opts ConfigureOptions) error {
	configPath := h.configPath()

	settings, err := mergeYAMLConfigFile(configPath, map[string]any{
		"model": map[string]any{
			"default":  opts.Model,
			"provider": hermesProvider,
			"default_headers": map[string]any{
				"HTTP-Referer":   "https://hermes-agent.nousresearch.com",
				"X-Origin-Title": "Hermes",
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge config file: %w", err)
	}

	// custom_providers is a list, so the Requesty entry is upserted by name
	// instead of being merged as a map.
	providers, err := hermesCustomProviders(settings)
	if err != nil {
		return err
	}
	settings["custom_providers"] = h.upsertCustomProvider(providers)

	if err := backupAndWriteConfigFileAsYAML(configPath, &settings); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (h *HermesHarness) configureOverwrite(opts ConfigureOptions) error {
	settings := hermesConfig{
		Model: hermesModel{
			Default:  opts.Model,
			Provider: hermesProvider,
			DefaultHeaders: map[string]string{
				"HTTP-Referer":   "https://hermes-agent.nousresearch.com",
				"X-Origin-Title": "Hermes",
			},
		},
		CustomProviders: []hermesCustomProvider{
			{
				Name:    hermesProvider,
				BaseURL: h.config.RouterBaseURL,
				APIKey:  h.config.APIKey,
				APIMode: hermesAPIMode,
			},
		},
	}

	if err := backupAndWriteConfigFileAsYAML(h.configPath(), &settings); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// upsertCustomProvider replaces the Requesty entry in providers, appending it
// when the user has not configured Requesty yet.
func (h *HermesHarness) upsertCustomProvider(providers []any) []any {
	provider := map[string]any{
		"name":     hermesProvider,
		"base_url": h.config.RouterBaseURL,
		"api_key":  h.config.APIKey,
		"api_mode": hermesAPIMode,
	}

	for i, existing := range providers {
		existingMap, ok := existing.(map[string]any)
		if !ok {
			continue
		}
		if existingMap["name"] != hermesProvider {
			continue
		}

		if err := mergePatch(existingMap, provider, "custom_providers"); err != nil {
			providers[i] = provider
			return providers
		}

		providers[i] = existingMap
		return providers
	}

	return append(providers, provider)
}

func hermesCustomProviders(settings map[string]any) ([]any, error) {
	value, exists := settings["custom_providers"]
	if !exists || value == nil {
		return nil, nil
	}

	providers, ok := value.([]any)
	if !ok {
		return nil, fmt.Errorf("expected field %q to be a list", "custom_providers")
	}

	return providers, nil
}

func (h *HermesHarness) configPath() string {
	return filepath.Join(h.configDir, "config.yaml")
}
