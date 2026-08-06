package harnesses

import "github.com/requestyai/cli/internal/config"

type Status struct {
	Files      []string
	Executable bool
	Configured bool
}

type ConfigureOptions struct {
	Model string
}

type Harness interface {
	Name() string
	Description() []string
	Status() (Status, error)
	Configure(ConfigureOptions) error
}

func Harnesses(config config.Config) []Harness {
	return []Harness{
		NewClaudeHarness(config),
		NewCodexHarness(config),
	}
}
