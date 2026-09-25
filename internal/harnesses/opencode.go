package harnesses

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

// NewOpenCodeHarness is OpenCode at its default global configuration
// directory.
func NewOpenCodeHarness(config config.Config) (Harness, error) {
	configDir, err := configDirInHome(".config", "opencode")
	if err != nil {
		return nil, err
	}

	return newOpenCodeHarness(config, configDir), nil
}

func newOpenCodeHarness(config config.Config, configDir string) *OpenCodeHarness {
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

	if path, err := exec.LookPath("opencode"); err == nil {
		status.Executable = true
		status.ExecutablePath = path
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

// openCodeModelFlags are the OpenCode flags that, when the user passes them,
// mean we leave the model alone.
var openCodeModelFlags = []string{"-m", "--model"}

// Launch replaces this process with OpenCode pointed at Requesty. OpenCode
// knows Requesty as a built-in provider that switches on when
// REQUESTY_API_KEY is set; an inline OPENCODE_CONFIG_CONTENT document, which
// outranks opencode.json for this run only, pins the base URL and key to
// this profile. Nothing is written to disk.
func (o *OpenCodeHarness) Launch(opts LaunchOptions) error {
	status, err := o.Status()
	if err != nil {
		return fmt.Errorf("failed to check %s: %w", o.Name(), err)
	}
	if !status.Executable {
		return fmt.Errorf("`opencode` is not on PATH; install OpenCode (https://opencode.ai/docs) and try again")
	}

	env := parseEnvironmentVariables(opts.Env)
	env["REQUESTY_API_KEY"] = o.config.APIKey
	content, err := o.inlineConfig(env["OPENCODE_CONFIG_CONTENT"], opts.Model)
	if err != nil {
		return err
	}
	env["OPENCODE_CONFIG_CONTENT"] = content

	if warning, conflict := o.credentialConflict(env); conflict {
		_, _ = fmt.Fprintln(os.Stderr, warning)
	}

	argv := []string{"opencode"}
	if opts.Model != "" && !hasAnyFlag(opts.Args, openCodeModelFlags) {
		argv = append(argv, "-m", o.modelID(opts.Model))
	}
	argv = append(argv, opts.Args...)

	if err := execProcess(status.ExecutablePath, argv, formatEnvironmentVariables(env)); err != nil {
		return fmt.Errorf("failed to launch opencode: %w", err)
	}

	return nil
}

// inlineConfig is the OPENCODE_CONFIG_CONTENT document for this run: the
// user's own, if they set one, with our provider merged in. The key is
// referenced by environment variable so it stays out of the document itself.
// The launched model is declared under the provider so ids OpenCode's
// catalog does not list, such as managed policies, still resolve.
func (o *OpenCodeHarness) inlineConfig(existing, model string) (string, error) {
	settings := make(map[string]any)
	if strings.TrimSpace(existing) != "" {
		decoder := json.NewDecoder(bytes.NewReader([]byte(existing)))
		decoder.UseNumber()
		if err := decoder.Decode(&settings); err != nil {
			return "", fmt.Errorf("OPENCODE_CONFIG_CONTENT is not valid JSON, so Requesty cannot add its provider to it; fix or unset it and try again: %w", err)
		}
	}

	provider := map[string]any{
		"options": map[string]any{
			"baseURL": o.baseURL(),
			"apiKey":  "{env:REQUESTY_API_KEY}",
			"headers": map[string]any{
				"X-Title": "OpenCode",
			},
		},
	}
	if model != "" {
		provider["models"] = map[string]any{model: map[string]any{}}
	}

	if err := mergePatch(settings, map[string]any{"provider": map[string]any{openCodeProvider: provider}}, ""); err != nil {
		return "", fmt.Errorf("OPENCODE_CONFIG_CONTENT cannot take a Requesty provider: %w", err)
	}

	data, err := json.Marshal(settings)
	if err != nil {
		return "", fmt.Errorf("failed to encode inline config: %w", err)
	}

	return string(data), nil
}

// credentialConflict reports a Requesty key stored by `opencode auth login`
// that differs from this profile's, since a stored credential can win over
// the one we inject and then fail authentication.
func (o *OpenCodeHarness) credentialConflict(env map[string]string) (string, bool) {
	authPath := openCodeAuthPath(env)
	if authPath == "" {
		return "", false
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		return "", false
	}

	var auth map[string]struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal(data, &auth); err != nil {
		return "", false
	}

	stored, ok := auth[openCodeProvider]
	if !ok || stored.Key == "" || stored.Key == o.config.APIKey {
		return "", false
	}

	return fmt.Sprintf("Warning: OpenCode has a Requesty credential stored at %s that differs from this profile's key. If requests fail to authenticate, remove it with `opencode auth logout` and try again.", authPath), true
}

// openCodeAuthPath is where `opencode auth login` keeps credentials:
// $XDG_DATA_HOME/opencode/auth.json, defaulting to ~/.local/share.
func openCodeAuthPath(env map[string]string) string {
	dataHome := strings.TrimSpace(env["XDG_DATA_HOME"])
	if dataHome == "" {
		home := env["HOME"]
		if home == "" {
			var err error
			if home, err = os.UserHomeDir(); err != nil {
				return ""
			}
		}
		dataHome = filepath.Join(home, ".local", "share")
	}

	return filepath.Join(dataHome, "opencode", "auth.json")
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
					"headers": map[string]any{
						"X-Title": "OpenCode",
					},
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
					Headers: map[string]string{
						"X-Title": "OpenCode",
					},
				},
			},
		},
	}

	if err := backupAndWriteConfigFileAsJSON(o.configPath(), &settings); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
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
