package cmd

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/spf13/cobra"
)

const (
	harnessProfileFlag         = "--" + profileFlag
	harnessModelFlag           = "--model"
	harnessChooseModelFlag     = "--choose-model"
	harnessFastModelFlag       = "--fast-model"
	harnessChooseFastModelFlag = "--choose-fast-model"
)

// harnessValueFlags are our flags that take a value, as `--flag v` or
// `--flag=v`.
var harnessValueFlags = []string{harnessProfileFlag, harnessModelFlag, harnessFastModelFlag}

// errHarnessHelp signals that the leading flags asked for help.
var errHarnessHelp = errors.New("help requested")

// harnessSpec is one `requesty <binary>` command.
type harnessSpec struct {
	// binary is the command name and the executable it launches.
	binary string
	// displayName is how the harness is referred to in messages.
	displayName string
	// defaultModels are the model ids to launch with when nothing has been
	// picked yet, most preferred first; the first one the profile can
	// route to is used without asking.
	defaultModels []string
	// defaultFastModels is the same for the model background work goes
	// to. Empty for harnesses without such a slot.
	defaultFastModels []string
	newHarness        func(config.Config) (harnesses.Harness, error)
}

// hasFastModel reports whether the harness hands background work to a
// second, smaller model.
func (s harnessSpec) hasFastModel() bool {
	return len(s.defaultFastModels) > 0
}

// newHarnessCommands returns the `requesty <harness>` commands, each of which
// starts a harness with Requesty injected for that run.
func newHarnessCommands(env *environment) []*cobra.Command {
	return []*cobra.Command{
		newHarnessCommand(env, harnessSpec{
			binary:            "claude",
			displayName:       "Claude Code",
			defaultModels:     []string{"claude-sonnet-4-6"},
			defaultFastModels: []string{"claude-haiku-4-5"},
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirClaudeCode()
				if err != nil {
					return nil, err
				}
				return harnesses.NewClaudeHarness(cfg, dir), nil
			},
		}),
		newHarnessCommand(env, harnessSpec{
			binary:        "codex",
			displayName:   "Codex",
			defaultModels: []string{"gpt-5.5"},
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirCodex()
				if err != nil {
					return nil, err
				}
				return harnesses.NewCodexHarness(cfg, dir), nil
			},
		}),
		newHarnessCommand(env, harnessSpec{
			binary:        "opencode",
			displayName:   "OpenCode",
			defaultModels: []string{"claude-sonnet-4-6"},
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirOpenCode()
				if err != nil {
					return nil, err
				}
				return harnesses.NewOpenCodeHarness(cfg, dir), nil
			},
		}),
		newHarnessCommand(env, harnessSpec{
			binary:        "pi",
			displayName:   "Pi",
			defaultModels: []string{"claude-sonnet-4-6"},
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirPi()
				if err != nil {
					return nil, err
				}
				return harnesses.NewPiHarness(cfg, dir), nil
			},
		}),
		newHarnessCommand(env, harnessSpec{
			binary:        "hermes",
			displayName:   "Hermes",
			defaultModels: []string{"claude-sonnet-4-6"},
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirHermes()
				if err != nil {
					return nil, err
				}
				return harnesses.NewHermesHarness(cfg, dir), nil
			},
		}),
	}
}

// newHarnessCommand builds `requesty <binary>`. The harness is constructed
// lazily, from the profile as it stands after any onboarding.
func newHarnessCommand(env *environment, spec harnessSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   harnessUse(spec),
		Short: fmt.Sprintf("Launch %s through Requesty", spec.displayName),
		Long:  harnessLong(spec),
		// We take over parsing so harness flags are never interpreted as ours.
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := parseHarnessArgs(args, spec)
			if errors.Is(err, errHarnessHelp) {
				return cmd.Help()
			}
			if err != nil {
				return err
			}

			cfg, err := env.ensureProfile(cmd, parsed.profile, spec.displayName)
			if err != nil {
				return err
			}

			harness, err := spec.newHarness(cfg)
			if err != nil {
				return fmt.Errorf("failed to set up %s: %w", spec.displayName, err)
			}

			parsed.launch.Model, parsed.launch.FastModel, err = env.ensureModels(cmd, cfg, spec, parsed)
			if err != nil {
				return err
			}

			return harness.Launch(parsed.launch)
		},
	}

	return cmd
}

// harnessUse is the one-line synopsis of `requesty <binary>`.
func harnessUse(spec harnessSpec) string {
	use := fmt.Sprintf("%s [%s <name>] [%s <id> | %s]", spec.binary, harnessProfileFlag, harnessModelFlag, harnessChooseModelFlag)
	if spec.hasFastModel() {
		use += fmt.Sprintf(" [%s <id> | %s]", harnessFastModelFlag, harnessChooseFastModelFlag)
	}

	return use + fmt.Sprintf(" [-- ] [%s args...]", spec.binary)
}

// harnessLong is the help page of `requesty <binary>`.
func harnessLong(spec harnessSpec) string {
	flags := [][2]string{
		{harnessProfileFlag + " <name>", "Saved profile to run as"},
		{harnessModelFlag + " <id>", "Model for this run only (a managed policy or any Requesty model id)"},
		{harnessChooseModelFlag, "Pick the model again and remember it"},
	}
	if spec.hasFastModel() {
		flags = append(flags,
			[2]string{harnessFastModelFlag + " <id>", "Model for background work, this run only"},
			[2]string{harnessChooseFastModelFlag, "Pick the background model again and remember it"},
		)
	}
	flags = append(flags, [2]string{"-h, --help", "Show this help"})

	var b strings.Builder
	fmt.Fprintf(&b, "Launch %s through Requesty for this run; its own configuration is not changed.\n\n", spec.displayName)
	b.WriteString("Flags:\n")
	for _, flag := range flags {
		fmt.Fprintf(&b, "  %-28s %s\n", flag[0], flag[1])
	}
	fmt.Fprintf(&b, "\nAnything else, or everything after `--`, is passed to `%s` untouched.\n\n", spec.binary)
	fmt.Fprintf(&b, "The first launch settles the model: %s is used when the profile can route to it,\n", spec.defaultModels[0])
	fmt.Fprintf(&b, "else a picker asks. The answer is remembered in the profile.")
	if spec.hasFastModel() {
		fmt.Fprintf(&b, " Background work is settled\nthe same way, preferring %s.", spec.defaultFastModels[0])
	}
	b.WriteString("\n")

	return b.String()
}

// harnessArgs is a harness command line split into our profile choice and
// what the harness is launched with.
type harnessArgs struct {
	profile         string
	chooseModel     bool
	chooseFastModel bool
	launch          harnesses.LaunchOptions
}

// parseHarnessArgs splits our leading flags from the harness's own arguments.
// A bare `--` ends our flags explicitly; otherwise the first token we do not
// recognise does. That keeps `requesty claude --model x -p "hi"` and
// `requesty claude -p "hi"` both working, and lets a harness flag that happens
// to share a name with ours (`codex --model`) still reach the harness when it
// appears later. spec says which of our flags apply to this harness.
func parseHarnessArgs(args []string, spec harnessSpec) (harnessArgs, error) {
	var parsed harnessArgs

flags:
	for len(args) > 0 {
		arg := args[0]

		switch {
		case arg == "--":
			parsed.launch.Args = args[1:]
			break flags

		case arg == "-h" || arg == "--help":
			return parsed, errHarnessHelp

		case arg == harnessChooseModelFlag:
			parsed.chooseModel = true
			args = args[1:]

		case arg == harnessChooseFastModelFlag:
			parsed.chooseFastModel = true
			args = args[1:]

		case slices.Contains(harnessValueFlags, arg):
			if len(args) < 2 {
				return parsed, fmt.Errorf("%s requires a value", arg)
			}
			if err := parsed.set(arg, args[1]); err != nil {
				return parsed, err
			}
			args = args[2:]

		case isValueFlagWithEquals(arg):
			flag, value, _ := strings.Cut(arg, "=")
			if err := parsed.set(flag, value); err != nil {
				return parsed, err
			}
			args = args[1:]

		default:
			parsed.launch.Args = args
			break flags
		}
	}

	if parsed.chooseModel && parsed.launch.Model != "" {
		return parsed, fmt.Errorf("%s and %s cannot be combined", harnessModelFlag, harnessChooseModelFlag)
	}
	if parsed.chooseFastModel && parsed.launch.FastModel != "" {
		return parsed, fmt.Errorf("%s and %s cannot be combined", harnessFastModelFlag, harnessChooseFastModelFlag)
	}
	if !spec.hasFastModel() {
		if parsed.launch.FastModel != "" {
			return parsed, fmt.Errorf("%s has no separate model for background work; %s does not apply", spec.displayName, harnessFastModelFlag)
		}
		if parsed.chooseFastModel {
			return parsed, fmt.Errorf("%s has no separate model for background work; %s does not apply", spec.displayName, harnessChooseFastModelFlag)
		}
	}

	return parsed, nil
}

// isValueFlagWithEquals reports whether arg is one of ours in `--flag=value`
// form.
func isValueFlagWithEquals(arg string) bool {
	for _, flag := range harnessValueFlags {
		if strings.HasPrefix(arg, flag+"=") {
			return true
		}
	}

	return false
}

func (p *harnessArgs) set(flag, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s requires a value", flag)
	}

	switch flag {
	case harnessProfileFlag:
		p.profile = value
	case harnessModelFlag:
		p.launch.Model = value
	case harnessFastModelFlag:
		p.launch.FastModel = value
	}

	return nil
}
