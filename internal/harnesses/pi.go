package harnesses

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/requestyai/cli/internal/client"
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

// NewPiHarness is Pi at its default agent configuration directory.
func NewPiHarness(config config.Config) (Harness, error) {
	configDir, err := configDirInHome(".pi", "agent")
	if err != nil {
		return nil, err
	}

	return newPiHarness(config, configDir), nil
}

func newPiHarness(config config.Config, configDir string) *PiHarness {
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

	if path, err := exec.LookPath("pi"); err == nil {
		status.Executable = true
		status.ExecutablePath = path
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

// piExtension registers Requesty as a Pi provider for one run. It is written
// out before every launch so an upgraded CLI always ships its current copy.
//
//go:embed pi/requesty.ts
var piExtension []byte

const (
	piExtensionFile = "requesty.ts"
	piCatalogFile   = "catalog.json"

	// piCatalogTimeout bounds the model list refresh so a slow network
	// cannot hold up the launch for long.
	piCatalogTimeout = 5 * time.Second

	// piCatalogContextWindow and piCatalogMaxTokens stand in when the model
	// list does not say.
	piCatalogContextWindow = 200_000
	piCatalogMaxTokens     = 8_192
)

// piCatalog is what the extension reads: the models the profile can route
// to, in the shape of Pi's ProviderModelConfig.
type piCatalog struct {
	Models []piCatalogModel `json:"models"`
}

type piCatalogModel struct {
	ID            string        `json:"id"`
	Name          string        `json:"name"`
	Reasoning     bool          `json:"reasoning"`
	Input         []string      `json:"input"`
	Cost          piCatalogCost `json:"cost"`
	ContextWindow int           `json:"contextWindow"`
	MaxTokens     int           `json:"maxTokens"`
}

// piCatalogCost is in dollars per million tokens, as Pi expects.
type piCatalogCost struct {
	Input      float64 `json:"input"`
	Output     float64 `json:"output"`
	CacheRead  float64 `json:"cacheRead"`
	CacheWrite float64 `json:"cacheWrite"`
}

// piProviderFlags and piModelFlags are the Pi flags that, when the user
// passes them, mean we leave the model choice alone.
var (
	piProviderFlags = []string{"--provider"}
	piModelFlags    = []string{"--model"}
)

// piNeutralizedEnv are the credentials Pi reads for its built-in providers.
// They are dropped from the launched process so every model Pi offers goes
// through Requesty rather than straight to a vendor.
var piNeutralizedEnv = []string{
	"AI_GATEWAY_API_KEY",
	"ANT_LING_API_KEY",
	"ANTHROPIC_API_KEY",
	"ANTHROPIC_AUTH_TOKEN",
	"ANTHROPIC_OAUTH_TOKEN",
	"AWS_BEARER_TOKEN_BEDROCK",
	"AZURE_OPENAI_API_KEY",
	"BASETEN_API_KEY",
	"CEREBRAS_API_KEY",
	"CLOUDFLARE_API_KEY",
	"DEEPSEEK_API_KEY",
	"FIREWORKS_API_KEY",
	"GEMINI_API_KEY",
	"GROQ_API_KEY",
	"KIMI_API_KEY",
	"MINIMAX_API_KEY",
	"MISTRAL_API_KEY",
	"MOONSHOT_API_KEY",
	"NVIDIA_API_KEY",
	"OPENAI_API_KEY",
	"OPENCODE_API_KEY",
	"OPENROUTER_API_KEY",
	"QWEN_TOKEN_PLAN_API_KEY",
	"QWEN_TOKEN_PLAN_CN_API_KEY",
	"TOGETHER_API_KEY",
	"XAI_API_KEY",
	"XIAOMI_API_KEY",
	"XIAOMI_TOKEN_PLAN_AMS_API_KEY",
	"XIAOMI_TOKEN_PLAN_CN_API_KEY",
	"XIAOMI_TOKEN_PLAN_SGP_API_KEY",
	"ZAI_API_KEY",
	"ZAI_CODING_CN_API_KEY",
}

// Launch replaces this process with Pi pointed at Requesty. Pi has no flag
// or variable for a custom provider, so a small extension of ours, loaded
// with --extension, registers Requesty with the models the profile can use.
// The extension and its model catalog live under ~/.requesty/pi; Pi's own
// configuration is not touched.
func (p *PiHarness) Launch(opts LaunchOptions) error {
	status, err := p.Status()
	if err != nil {
		return fmt.Errorf("failed to check %s: %w", p.Name(), err)
	}
	if !status.Executable {
		return fmt.Errorf("`pi` is not on PATH; install Pi (https://pi.dev) and try again")
	}

	launchDir, err := requestyDirInHome("pi")
	if err != nil {
		return fmt.Errorf("failed to find launch directory: %w", err)
	}
	extensionPath := filepath.Join(launchDir, piExtensionFile)
	catalogPath := filepath.Join(launchDir, piCatalogFile)

	if err := writeFile(extensionPath, piExtension, 0o600); err != nil {
		return fmt.Errorf("failed to write Pi extension: %w", err)
	}
	if err := p.writeCatalog(catalogPath, opts.Model); err != nil {
		return fmt.Errorf("failed to write Pi model catalog: %w", err)
	}

	env := parseEnvironmentVariables(opts.Env)
	for _, key := range piNeutralizedEnv {
		delete(env, key)
	}
	env["REQUESTY_API_KEY"] = p.config.APIKey
	env["REQUESTY_BASE_URL"] = p.config.RouterBaseURL
	env["REQUESTY_PI_CATALOG"] = catalogPath

	argv := []string{"pi", "--extension", extensionPath}
	switch {
	case hasAnyFlag(opts.Args, piModelFlags), hasAnyFlag(opts.Args, piProviderFlags):
		// The user has chosen for themselves.
	case opts.Model != "":
		argv = append(argv, "--model", piProvider+"/"+opts.Model)
	default:
		argv = append(argv, "--provider", piProvider)
	}
	argv = append(argv, opts.Args...)

	if err := execProcess(status.ExecutablePath, argv, formatEnvironmentVariables(env)); err != nil {
		return fmt.Errorf("failed to launch pi: %w", err)
	}

	return nil
}

// writeCatalog refreshes the model catalog from the profile's account. When
// that fails, such as offline, the previous catalog is kept. Either way the
// launched model is listed, since Pi only accepts models it knows about.
func (p *PiHarness) writeCatalog(path, model string) error {
	ctx, cancel := context.WithTimeout(context.Background(), piCatalogTimeout)
	defer cancel()

	catalog, err := p.fetchCatalog(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Warning: could not refresh the Requesty model list for Pi (%v); using the previous list.\n", err)
		catalog = readPiCatalog(path)
	}

	if model != "" && !slices.ContainsFunc(catalog.Models, func(m piCatalogModel) bool { return m.ID == model }) {
		catalog.Models = append(catalog.Models, piCatalogModel{
			ID:            model,
			Name:          model,
			Reasoning:     true,
			Input:         []string{"text", "image"},
			ContextWindow: piCatalogContextWindow,
			MaxTokens:     piCatalogMaxTokens,
		})
	}

	data, err := json.MarshalIndent(catalog, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode catalog: %w", err)
	}

	return writeFile(path, data, 0o600)
}

// fetchCatalog lists the profile's models and managed policies. Policies
// are optional: an account without any still gets its models.
func (p *PiHarness) fetchCatalog(ctx context.Context) (piCatalog, error) {
	apiClient := client.New(p.config)

	models, err := apiClient.Models(ctx)
	if err != nil {
		return piCatalog{}, err
	}
	policies, _ := apiClient.ManagedPolicies(ctx)

	listed := slices.Concat(policies, models)
	slices.SortFunc(listed, func(a, b client.Model) int { return strings.Compare(a.ID, b.ID) })

	catalog := piCatalog{Models: make([]piCatalogModel, 0, len(listed))}
	for _, model := range listed {
		if model.ID == "" || slices.ContainsFunc(catalog.Models, func(m piCatalogModel) bool { return m.ID == model.ID }) {
			continue
		}
		catalog.Models = append(catalog.Models, piCatalogModelFrom(model))
	}

	return catalog, nil
}

// piCatalogModelFrom describes a Requesty model to Pi. Prices arrive per
// token and leave per million; capacities the list omits get defaults.
func piCatalogModelFrom(model client.Model) piCatalogModel {
	entry := piCatalogModel{
		ID:        model.ID,
		Name:      model.ID,
		Reasoning: true,
		Input:     []string{"text", "image"},
		Cost: piCatalogCost{
			Input:      model.InputPrice * 1_000_000,
			Output:     model.OutputPrice * 1_000_000,
			CacheRead:  model.CacheReadPrice * 1_000_000,
			CacheWrite: model.CacheWritePrice * 1_000_000,
		},
		ContextWindow: model.ContextWindow,
		MaxTokens:     model.MaxOutputTokens,
	}
	if entry.ContextWindow <= 0 {
		entry.ContextWindow = piCatalogContextWindow
	}
	if entry.MaxTokens <= 0 {
		entry.MaxTokens = piCatalogMaxTokens
	}

	return entry
}

// readPiCatalog returns the catalog at path, or an empty one when there is
// none or it cannot be read.
func readPiCatalog(path string) piCatalog {
	var catalog piCatalog
	data, err := os.ReadFile(path)
	if err != nil {
		return catalog
	}
	if err := json.Unmarshal(data, &catalog); err != nil {
		return piCatalog{}
	}

	return catalog
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
