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
	deepseekProvider = "requesty"

	// deepseekAPI is the OpenAI Chat Completions format, which the harness
	// speaks against a base URL that keeps the /v1 suffix.
	deepseekAPI = "openai-completions"

	// deepseekCredentialRef names the credential in the harness credentials
	// file. The settings file references it instead of holding the key.
	deepseekCredentialRef = "REQUESTY_API_KEY"

	deepseekContextWindow = 200000
	deepseekMaxTokens     = 8192
)

type deepseekSettings struct {
	DefaultModel deepseekDefaultModel `yaml:"agent-default-model"`
	PiAI         deepseekPiAI         `yaml:"llm-pi-ai"`
}

type deepseekDefaultModel struct {
	Provider string `yaml:"provider"`
	Model    string `yaml:"model"`
}

type deepseekPiAI struct {
	Providers map[string]deepseekProviderConfig `yaml:"providers"`
}

type deepseekProviderConfig struct {
	DisplayName          string                `yaml:"displayName"`
	APIKeyEnv            string                `yaml:"apiKeyEnv"`
	API                  string                `yaml:"api"`
	BaseURL              string                `yaml:"baseURL"`
	DefaultContextWindow int                   `yaml:"defaultContextWindow"`
	DefaultMaxTokens     int                   `yaml:"defaultMaxTokens"`
	Headers              map[string]string     `yaml:"headers,omitempty"`
	Models               []deepseekModelConfig `yaml:"models"`
}

type deepseekModelConfig struct {
	ID string `yaml:"id"`
}

// deepseekRawSettings reads the model entries without dropping fields the
// harness writes for them, such as display names and input capacities.
type deepseekRawSettings struct {
	PiAI struct {
		Providers map[string]struct {
			Models []map[string]any `yaml:"models"`
		} `yaml:"providers"`
	} `yaml:"llm-pi-ai"`
}

type DeepSeekHarness struct {
	config    config.Config
	configDir string
}

// DefaultConfigDirDeepSeek is the DeepSeek Harness home, which holds its
// settings and credentials. The harness reads DSH_HOME first, so an explicit
// home wins over the default one.
func DefaultConfigDirDeepSeek() (string, error) {
	if home := os.Getenv("DSH_HOME"); home != "" {
		return home, nil
	}

	return configDirInHome(".dsh")
}

func NewDeepSeekHarness(config config.Config, configDir string) *DeepSeekHarness {
	return &DeepSeekHarness{
		config:    config,
		configDir: configDir,
	}
}

func (d *DeepSeekHarness) Name() string {
	return "DeepSeek Harness"
}

func (d *DeepSeekHarness) Description() []string {
	return []string{
		"takes a backup of settings.yaml and .credentials.yaml",
		"writes settings.yaml and .credentials.yaml to route through Requesty",
	}
}

func (d *DeepSeekHarness) Status() (Status, error) {
	status := Status{}

	if _, err := exec.LookPath("dsh"); err == nil {
		status.Executable = true
	} else if !errors.Is(err, exec.ErrNotFound) {
		return status, fmt.Errorf("failed to find executable: %w", err)
	}

	settingsPath := d.settingsPath()
	credentialsPath := d.credentialsPath()
	status.Files = append(status.Files, settingsPath, credentialsPath)

	settingsExists, err := pathExists(settingsPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	credentialsExists, err := pathExists(credentialsPath)
	if err != nil {
		return status, fmt.Errorf("failed to check file exists: %w", err)
	}

	if !settingsExists || !credentialsExists {
		status.Configured = false
		return status, nil
	}

	settingsBytes, err := os.ReadFile(settingsPath)
	if err != nil {
		return status, fmt.Errorf("failed to read file: %w", err)
	}

	var settings deepseekSettings
	if err := yaml.Unmarshal(settingsBytes, &settings); err != nil {
		return status, fmt.Errorf("failed to unmarshal: %w", err)
	}

	credentialsBytes, err := os.ReadFile(credentialsPath)
	if err != nil {
		return status, fmt.Errorf("failed to read credentials file: %w", err)
	}

	credentials := make(map[string]string)
	if err := yaml.Unmarshal(credentialsBytes, &credentials); err != nil {
		return status, fmt.Errorf("failed to unmarshal credentials: %w", err)
	}

	status.Configured = settings.PiAI.Providers[deepseekProvider].BaseURL == d.routerBaseURL() &&
		settings.DefaultModel.Provider == deepseekProvider &&
		credentials[deepseekCredentialRef] != ""

	return status, nil
}

func (d *DeepSeekHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return d.configureOverwrite(opts)
	}

	return d.configureMerge(opts)
}

func (d *DeepSeekHarness) configureMerge(opts ConfigureOptions) error {
	settingsPath := d.settingsPath()

	models, err := d.mergedModels(settingsPath, opts.Model)
	if err != nil {
		return err
	}

	settings, err := mergeOrCreateYAMLConfigFile(settingsPath, map[string]any{
		"agent-default-model": map[string]any{
			"provider": deepseekProvider,
			"model":    opts.Model,
		},
		"llm-pi-ai": map[string]any{
			"providers": map[string]any{
				deepseekProvider: map[string]any{
					"displayName":          "Requesty",
					"apiKeyEnv":            deepseekCredentialRef,
					"api":                  deepseekAPI,
					"baseURL":              d.routerBaseURL(),
					"defaultContextWindow": deepseekContextWindow,
					"defaultMaxTokens":     deepseekMaxTokens,
					"headers":              headerPatch(d.headers(opts)),
					"models":               models,
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to merge settings file: %w", err)
	}

	if err := backupAndWriteConfigFileAsYAML(settingsPath, &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	if err := d.writeCredentials(); err != nil {
		return err
	}

	return nil
}

func (d *DeepSeekHarness) configureOverwrite(opts ConfigureOptions) error {
	settings := deepseekSettings{
		DefaultModel: deepseekDefaultModel{
			Provider: deepseekProvider,
			Model:    opts.Model,
		},
		PiAI: deepseekPiAI{
			Providers: map[string]deepseekProviderConfig{
				deepseekProvider: {
					DisplayName:          "Requesty",
					APIKeyEnv:            deepseekCredentialRef,
					API:                  deepseekAPI,
					BaseURL:              d.routerBaseURL(),
					DefaultContextWindow: deepseekContextWindow,
					DefaultMaxTokens:     deepseekMaxTokens,
					Headers:              d.headers(opts),
					Models: []deepseekModelConfig{
						{ID: opts.Model},
					},
				},
			},
		},
	}

	if err := backupAndWriteConfigFileAsYAML(d.settingsPath(), &settings); err != nil {
		return fmt.Errorf("failed to write settings file: %w", err)
	}

	if err := d.writeCredentials(); err != nil {
		return err
	}

	return nil
}

// mergedModels keeps the model entries already listed for the Requesty
// provider, including any the user added through the harness itself, and adds
// the selected model when it is missing. The harness needs an explicit catalog
// because a custom provider ships none, so the list cannot be dropped, and a
// later run must not discard models the user picked in the harness.
func (d *DeepSeekHarness) mergedModels(settingsPath, model string) ([]any, error) {
	settingsBytes, err := os.ReadFile(settingsPath)
	if errors.Is(err, os.ErrNotExist) {
		return []any{map[string]any{"id": model}}, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to read settings file: %w", err)
	}

	var settings deepseekRawSettings
	if err := yaml.Unmarshal(settingsBytes, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings file: %w", err)
	}

	models := make([]any, 0, len(settings.PiAI.Providers[deepseekProvider].Models)+1)
	selected := false

	for _, entry := range settings.PiAI.Providers[deepseekProvider].Models {
		id, ok := entry["id"].(string)
		if !ok || id == "" {
			continue
		}

		if id == model {
			selected = true
		}

		models = append(models, entry)
	}

	if !selected {
		models = append(models, map[string]any{"id": model})
	}

	return models, nil
}

// writeCredentials stores the API key in the harness credentials file, which
// the settings file reaches through apiKeyEnv. Other stored credentials are
// preserved.
func (d *DeepSeekHarness) writeCredentials() error {
	credentialsPath := d.credentialsPath()

	credentials, err := mergeOrCreateYAMLConfigFile(credentialsPath, map[string]any{
		deepseekCredentialRef: d.config.APIKey,
	})
	if err != nil {
		return fmt.Errorf("failed to merge credentials file: %w", err)
	}

	if err := backupAndWriteConfigFileAsYAML(credentialsPath, &credentials); err != nil {
		return fmt.Errorf("failed to write credentials file: %w", err)
	}

	return nil
}

// headers name the harness to Requesty and carry the attribution dimensions it
// can be given upfront. The harness has no way to read a header value from the
// environment or a command, so the repository and branch are left out rather
// than frozen to wherever the CLI happened to run.
func (d *DeepSeekHarness) headers(opts ConfigureOptions) map[string]string {
	return opts.Attribution.Static(map[string]string{
		"HTTP-Referer": "https://requesty.ai",
		"X-Title":      "DeepSeek Harness",
	})
}

// routerBaseURL keeps the /v1 suffix the OpenAI Chat Completions format needs.
func (d *DeepSeekHarness) routerBaseURL() string {
	return fmt.Sprintf("%s/v1", d.config.RouterBaseURL)
}

func (d *DeepSeekHarness) settingsPath() string {
	return filepath.Join(d.configDir, "settings.yaml")
}

func (d *DeepSeekHarness) credentialsPath() string {
	return filepath.Join(d.configDir, ".credentials.yaml")
}
