package harnesses

import (
	"fmt"

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

	return []Harness{
		NewClaudeHarness(config, claudeCodeConfigDir),
		NewCodexHarness(config, codexConfigDir),
		NewOpenCodeHarness(config, openCodeConfigDir),
		NewPiHarness(config, piConfigDir),
		NewHermesHarness(config, hermesConfigDir),
	}, nil
}
