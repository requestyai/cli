package harnesses

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/requestyai/cli/internal/config"
)

const (
	claudeCodeConfigDirName = ".claude"
	codexConfigDirName      = ".codex"
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

	return []Harness{
		NewClaudeHarness(config, claudeCodeConfigDir),
		NewCodexHarness(config, codexConfigDir),
	}, nil
}

// DefaultConfigDirClaudeCode is where Claude Code keeps its configuration.
func DefaultConfigDirClaudeCode() (string, error) {
	return configDirInHome(claudeCodeConfigDirName)
}

// DefaultConfigDirCodex is where Codex keeps its configuration.
func DefaultConfigDirCodex() (string, error) {
	return configDirInHome(codexConfigDirName)
}

func configDirInHome(name string) (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home path: %w", err)
	}

	return filepath.Join(homePath, name), nil
}
