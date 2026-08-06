package harnesses

type Status struct {
	Files      []string
	Executable bool
	Configured bool
}

type ConfigureOptions struct {
	APIKey string
	Model  string
}

type Harness interface {
	Name() string
	Description() []string
	Status() (Status, error)
	Configure(ConfigureOptions) error
}

func Harnesses() []Harness {
	return []Harness{
		NewClaudeHarness(),
		NewCodexHarness(),
	}
}
