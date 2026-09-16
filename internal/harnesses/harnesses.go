package harnesses

import (
	"fmt"

	"github.com/requestyai/cli/internal/config"
)

type Status struct {
	Files      []string
	Executable bool

	ExecutablePath string
	Configured     bool
}

type ConfigureOptions struct {
	Model     string
	Overwrite bool
}

type LaunchOptions struct {
	// Model overrides the harness's default model for this run. Empty leaves
	// whatever the harness would pick on its own (including a model our
	// Configure wrote into its settings file earlier).
	Model string
	// Effort is one of Efforts, or empty for the harness default.
	Effort string
	// Args is passed to the harness binary untouched, after our own flags.
	Args []string
	// Env is the environment to start from. nil means os.Environ().
	Env []string
}

type Harness interface {
	Name() string
	Description() []string
	Status() (Status, error)
	Configure(ConfigureOptions) error
	Launch(LaunchOptions) error
}

func Harnesses(config config.Config) ([]Harness, error) {
	claudeCodeConfigDir, err := DefaultConfigDirClaudeCode()
	if err != nil {
		return nil, fmt.Errorf("failed to get Claude Code config directory: %w", err)
	}

	codexConfigDir, err := DefaultConfigDirCodex()
	if err != nil {
		return nil, fmt.Errorf("failed to get Codex config directory: %w", err)
	}

	openCodeConfigDir, err := DefaultConfigDirOpenCode()
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenCode config directory: %w", err)
	}

	piConfigDir, err := DefaultConfigDirPi()
	if err != nil {
		return nil, fmt.Errorf("failed to get Pi config directory: %w", err)
	}

	hermesConfigDir, err := DefaultConfigDirHermes()
	if err != nil {
		return nil, fmt.Errorf("failed to get Hermes config directory: %w", err)
	}

	deepseekConfigDir, err := DefaultConfigDirDeepSeek()
	if err != nil {
		return nil, fmt.Errorf("failed to get DeepSeek Harness config directory: %w", err)
	}

	return []Harness{
		NewClaudeHarness(config, claudeCodeConfigDir),
		NewCodexHarness(config, codexConfigDir),
		NewOpenCodeHarness(config, openCodeConfigDir),
		NewPiHarness(config, piConfigDir),
		NewHermesHarness(config, hermesConfigDir),
		NewDeepSeekHarness(config, deepseekConfigDir),
	}, nil
}
