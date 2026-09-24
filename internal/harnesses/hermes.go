package harnesses

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// DefaultConfigDirHermes is where Hermes keeps its configuration. Hermes
// reads HERMES_HOME first, so an explicit home wins over the default one.
func DefaultConfigDirHermes() (string, error) {
	if home := os.Getenv("HERMES_HOME"); home != "" {
		return home, nil
	}

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

	if path, err := exec.LookPath("hermes"); err == nil {
		status.Executable = true
		status.ExecutablePath = path
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

// hermesProviderFlags and hermesModelFlags are the Hermes flags that, when
// the user passes them, mean we leave the corresponding choice alone.
var (
	hermesProviderFlags = []string{"--provider"}
	hermesModelFlags    = []string{"-m", "--model"}
)

// hermesLaunchEnv are the variables the launch sets; a ~/.hermes/.env that
// redefines one of them wins over us, so they are what we check for.
var hermesLaunchEnv = []string{"CUSTOM_BASE_URL", "REQUESTY_API_KEY"}

// Launch replaces this process with Hermes pointed at Requesty. Hermes's
// `custom` provider takes its endpoint from CUSTOM_BASE_URL and derives the
// key variable from the endpoint's host, which for router.requesty.ai is
// REQUESTY_API_KEY. The custom provider speaks OpenAI chat completions, so
// the base URL keeps its /v1 suffix. Nothing is written to disk.
func (h *HermesHarness) Launch(opts LaunchOptions) error {
	status, err := h.Status()
	if err != nil {
		return fmt.Errorf("failed to check %s: %w", h.Name(), err)
	}
	if !status.Executable {
		return fmt.Errorf("`hermes` is not on PATH; install Hermes (https://hermes-agent.nousresearch.com/docs/getting-started/quickstart) and try again")
	}

	env := parseEnvironmentVariables(opts.Env)
	env["CUSTOM_BASE_URL"] = h.config.RouterBaseURL + "/v1"
	env["REQUESTY_API_KEY"] = h.config.APIKey

	if warning, conflict := h.credentialConflict(env); conflict {
		_, _ = fmt.Fprintln(os.Stderr, warning)
	}

	argv := []string{"hermes"}
	if !hasAnyFlag(opts.Args, hermesProviderFlags) {
		argv = append(argv, "--provider", "custom")
	}
	if opts.Model != "" && !hasAnyFlag(opts.Args, hermesModelFlags) {
		argv = append(argv, "-m", opts.Model)
	}
	argv = append(argv, opts.Args...)

	if err := execProcess(status.ExecutablePath, argv, formatEnvironmentVariables(env)); err != nil {
		return fmt.Errorf("failed to launch hermes: %w", err)
	}

	return nil
}

// credentialConflict reports a value in Hermes's .env file that would
// replace one of ours: Hermes loads that file over the process environment,
// so a stale entry there sends requests elsewhere or with another key.
func (h *HermesHarness) credentialConflict(env map[string]string) (string, bool) {
	envPath := h.envPath()

	data, err := os.ReadFile(envPath)
	if err != nil {
		return "", false
	}

	stored := parseDotEnv(string(data))
	conflicting := make([]string, 0, len(hermesLaunchEnv))
	for _, key := range hermesLaunchEnv {
		if value, ok := stored[key]; ok && value != env[key] {
			conflicting = append(conflicting, key)
		}
	}
	if len(conflicting) == 0 {
		return "", false
	}

	return fmt.Sprintf("Warning: %s in %s takes precedence over the values Requesty sets for this run. Remove or update it there if Hermes does not reach Requesty.", strings.Join(conflicting, " and "), envPath), true
}

// parseDotEnv reads KEY=value lines the way Hermes's dotenv loader does for
// the cases that matter here: comments and blanks are skipped, `export` is
// allowed, and matching quotes around the value are removed.
func parseDotEnv(content string) map[string]string {
	values := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")

		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if len(value) >= 2 && (value[0] == '"' || value[0] == '\'') && value[len(value)-1] == value[0] {
			value = value[1 : len(value)-1]
		}
		values[key] = value
	}

	return values
}

func (h *HermesHarness) envPath() string {
	return filepath.Join(h.configDir, ".env")
}

func (h *HermesHarness) Configure(opts ConfigureOptions) error {
	if opts.Overwrite {
		return h.configureOverwrite(opts)
	}

	return h.configureMerge(opts)
}

func (h *HermesHarness) configureMerge(opts ConfigureOptions) error {
	configPath := h.configPath()

	settings, err := mergeOrCreateYAMLConfigFile(configPath, map[string]any{
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
