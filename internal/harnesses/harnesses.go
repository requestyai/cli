package harnesses

import (
	"fmt"

	"github.com/requestyai/cli/internal/attribution"
	"github.com/requestyai/cli/internal/config"
)

type Status struct {
	Files      []string
	Executable bool
	Configured bool
}

type ConfigureOptions struct {
	Model     string
	Overwrite bool

	// Attribution is the set of dimensions to attribute requests by. The zero
	// value writes no attribution headers, which is what a user who did not opt
	// in gets.
	Attribution attribution.Set
}

type Harness interface {
	Name() string
	Description() []string
	Status() (Status, error)
	Configure(ConfigureOptions) error
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
