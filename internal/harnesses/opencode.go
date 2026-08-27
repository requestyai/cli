package harnesses

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/requestyai/cli/internal/attribution"
	"github.com/requestyai/cli/internal/config"
)

const (
	openCodeProvider = "requesty"
	openCodeSchema   = "https://opencode.ai/config.json"
)

type openCodeConfig struct {
	Schema    string                            `json:"$schema,omitempty"`
	Model     string                            `json:"model,omitempty"`
	Providers map[string]openCodeProviderConfig `json:"provider"`
}

type openCodeProviderConfig struct {
	Options openCodeProviderOptions `json:"options"`
}

type openCodeProviderOptions struct {
	BaseURL string            `json:"baseURL"`
	APIKey  string            `json:"apiKey"`
	Headers map[string]string `json:"headers,omitempty"`
}

type OpenCodeHarness struct {
	config    config.Config
	configDir string
}

// DefaultConfigDirOpenCode is where OpenCode keeps its global configuration.
func DefaultConfigDirOpenCode() (string, error) {
	return configDirInHome(".config", "opencode")
}

func NewOpenCodeHarness(config config.Config, configDir string) *OpenCodeHarness {
	return &OpenCodeHarness{
		config:    config,
		configDir: configDir,
	}
}

func (o *OpenCodeHarness) Name() string {
	return "OpenCode"
}

func (o *OpenCodeHarness) Description() []string {
	return []string{
		"takes a backup of opencode.json",
		"writes an opencode.json to route through Requesty",
	}
}

func (o *OpenCodeHarness) Status() (Status, error) {
	status := Status{}

	if _, err := exec.LookPath("opencode"); err == nil {
		status.Executable = true
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	configPath := o.configPath()
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

	var settings openCodeConfig
	if err := json.Unmarshal(configBytes, &settings); err != nil {
		return status, fmt.Errorf("failed to unmarshal: %w", err)
	}

	status.Configured = settings.Providers[openCodeProvider].Options.BaseURL == o.baseURL()

	return status, nil
}

func (o *OpenCodeHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return o.configureOverwrite(opts)
	}

	return o.configureMerge(opts)
}

func (o *OpenCodeHarness) configureMerge(opts ConfigureOptions) error {
	configPath := o.configPath()

	settings, err := mergeOrCreateJSONConfigFile(configPath, map[string]any{
		"$schema": openCodeSchema,
		"model":   o.modelID(opts.Model),
		"provider": map[string]any{
			openCodeProvider: map[string]any{
				"options": map[string]any{
					"baseURL": o.baseURL(),
					"apiKey":  o.config.APIKey,
					"headers": headerPatch(o.headers(opts)),
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge config file: %w", err)
	}

	if err := backupAndWriteConfigFileAsJSON(configPath, &settings); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

func (o *OpenCodeHarness) configureOverwrite(opts ConfigureOptions) error {
	settings := openCodeConfig{
		Schema: openCodeSchema,
		Model:  o.modelID(opts.Model),
		Providers: map[string]openCodeProviderConfig{
			openCodeProvider: {
				Options: openCodeProviderOptions{
					BaseURL: o.baseURL(),
					APIKey:  o.config.APIKey,
					Headers: o.headers(opts),
				},
			},
		},
	}

	if err := backupAndWriteConfigFileAsJSON(o.configPath(), &settings); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// headers name OpenCode to Requesty and carry the attribution dimensions.
// OpenCode expands an {env:…} placeholder anywhere in its config file, so the
// dynamic dimensions are read from what the shell hook exported.
func (o *OpenCodeHarness) headers(opts ConfigureOptions) map[string]string {
	return opts.Attribution.All(map[string]string{
		"X-Title": "OpenCode",
	}, attribution.EnvPlaceholder)
}

// modelID qualifies the model with the provider, as OpenCode addresses models
// as "<provider>/<model>".
func (o *OpenCodeHarness) modelID(model string) string {
	if model == "" {
		return ""
	}

	return fmt.Sprintf("%s/%s", openCodeProvider, model)
}

func (o *OpenCodeHarness) baseURL() string {
	return fmt.Sprintf("%s/v1", o.config.RouterBaseURL)
}

func (o *OpenCodeHarness) configPath() string {
	return filepath.Join(o.configDir, "opencode.json")
}
