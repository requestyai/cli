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
					"headers": map[string]any{
						"HTTP-Referer": "https://requesty.ai",
						"X-Title":      "DeepSeek Harness",
					},
					"models": []any{
						map[string]any{"id": opts.Model},
					},
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
					Headers: map[string]string{
						"HTTP-Referer": "https://requesty.ai",
						"X-Title":      "DeepSeek Harness",
					},
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
