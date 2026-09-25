package harnesses

import (
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
	// Model is what the harness is told to use: a managed policy or any
	// Requesty model id.
	Model string
	// FastModel is what the harness hands background work to, such as
	// Claude Code's haiku alias.
	FastModel string
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

// Harnesses builds every supported harness for the profile.
func Harnesses(cfg config.Config) ([]Harness, error) {
	constructors := []func(config.Config) (Harness, error){
		NewClaudeHarness,
		NewCodexHarness,
		NewOpenCodeHarness,
		NewPiHarness,
		NewHermesHarness,
		NewDeepSeekHarness,
	}

	harnesses := make([]Harness, 0, len(constructors))
	for _, constructor := range constructors {
		harness, err := constructor(cfg)
		if err != nil {
			return nil, err
		}
		harnesses = append(harnesses, harness)
	}

	return harnesses, nil
}

// LaunchSpecification is how `requesty <binary>` starts one harness. It needs
// no profile, so the commands can be registered before one has been picked.
type LaunchSpecification struct {
	// Binary is the executable the harness is started with, and the name of
	// the `requesty <binary>` command that does so.
	Binary string
	// Name is how the harness is referred to in messages.
	Name string
	// DefaultModels are the model ids to launch with when nothing has been
	// picked yet, most preferred first; the first one the profile can route
	// to is used without asking.
	DefaultModels []string
	// DefaultFastModels is the same for the model background work goes to.
	// Empty for harnesses without such a slot.
	DefaultFastModels []string
	// New builds the harness at its default configuration directory.
	New func(config.Config) (Harness, error)
}

// HasFastModel reports whether the harness hands background work to a
// second, smaller model.
func (s LaunchSpecification) HasFastModel() bool {
	return len(s.DefaultFastModels) > 0
}

// LaunchSpecifications is every harness `requesty` can launch, in the order
// the commands are listed.
var LaunchSpecifications = []LaunchSpecification{
	{
		Binary:            "claude",
		Name:              "Claude Code",
		DefaultModels:     []string{"claude-sonnet-5"},
		DefaultFastModels: []string{"claude-haiku-4-5"},
		New:               NewClaudeHarness,
	},
	{
		Binary:        "codex",
		Name:          "Codex",
		DefaultModels: []string{"gpt-6-sol"},
		New:           NewCodexHarness,
	},
	{
		Binary:        "opencode",
		Name:          "OpenCode",
		DefaultModels: []string{"claude-sonnet-5", "gpt-6-sol"},
		New:           NewOpenCodeHarness,
	},
	{
		Binary:        "pi",
		Name:          "Pi",
		DefaultModels: []string{"claude-sonnet-5", "gpt-6-sol"},
		New:           NewPiHarness,
	},
	{
		Binary:        "hermes",
		Name:          "Hermes",
		DefaultModels: []string{"claude-sonnet-5", "gpt-6-sol"},
		New:           NewHermesHarness,
	},
}
