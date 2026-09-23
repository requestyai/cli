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
	harnessProfileFlag     = "--" + profileFlag
	harnessModelFlag       = "--model"
	harnessChooseModelFlag = "--choose-model"
	harnessEffortFlag      = "--reasoning-effort"
)

// errHarnessHelp signals that the leading flags asked for help.
var errHarnessHelp = errors.New("help requested")

// harnessSpec is one `requesty <binary>` command.
type harnessSpec struct {
	// binary is the command name and the executable it launches.
	binary string
	// displayName is how the harness is referred to in messages.
	displayName string
	// defaultModel is the managed policy the picker suggests when nothing
	// has been picked yet.
	defaultModel string
	newHarness   func(config.Config) (harnesses.Harness, error)
}

// newHarnessCommands returns the `requesty <harness>` commands, each of which
// starts a harness with Requesty injected for that run.
func newHarnessCommands(env *environment) []*cobra.Command {
	return []*cobra.Command{
		newHarnessCommand(env, harnessSpec{
			binary:       "claude",
			displayName:  "Claude Code",
			defaultModel: "claude-sonnet-4-6",
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirClaudeCode()
				if err != nil {
					return nil, err
				}
				return harnesses.NewClaudeHarness(cfg, dir), nil
			},
		}),
		newHarnessCommand(env, harnessSpec{
			binary:       "codex",
			displayName:  "Codex",
			defaultModel: "gpt-5.5",
			newHarness: func(cfg config.Config) (harnesses.Harness, error) {
				dir, err := harnesses.DefaultConfigDirCodex()
				if err != nil {
					return nil, err
				}
				return harnesses.NewCodexHarness(cfg, dir), nil
			},
		}),
	}
}

// newHarnessCommand builds `requesty <binary>`. The harness is constructed
// lazily, from the profile as it stands after any onboarding, so a machine
// without a resolvable home directory still gets a help page and a clear
// error, rather than a missing command.
func newHarnessCommand(env *environment, spec harnessSpec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [--profile <name>] [--model <id> | --choose-model] [--reasoning-effort <level>] [-- ] [%s args...]", spec.binary, spec.binary),
		Short: fmt.Sprintf("Launch %s through Requesty", spec.displayName),
		Long: fmt.Sprintf("Launch %s with its traffic routed through Requesty for this run only.\n\n", spec.displayName) +
			"Flags:\n" +
			fmt.Sprintf("  %-28s %s\n", harnessProfileFlag+" <name>", "Saved profile to run as (also "+profileEnv+")") +
			fmt.Sprintf("  %-28s %s\n", harnessModelFlag+" <id>", "Model for this run only (a managed policy or any Requesty model id)") +
			fmt.Sprintf("  %-28s %s\n", harnessChooseModelFlag, "Pick the model to launch with from now on") +
			fmt.Sprintf("  %-28s %s\n", harnessEffortFlag+" <level>", "Reasoning effort: "+strings.Join(harnesses.Efforts, ", ")) +
			fmt.Sprintf("  %-28s %s\n\n", "-h, --help", "Show this help") +
			fmt.Sprintf("Anything else, or everything after `--`, is passed to `%s` untouched.\n\n", spec.binary) +
			"Without --model, the model picked for this harness in the profile is used. The first\n" +
			fmt.Sprintf("time, a picker asks (suggesting %s) and remembers the answer.\n\n", spec.defaultModel),
		// We take over parsing so harness flags are never interpreted as ours.
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := parseHarnessArgs(args)
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

			parsed.launch.Model, err = env.ensureModel(cmd, cfg, spec, parsed)
			if err != nil {
				return err
			}

			return harness.Launch(parsed.launch)
		},
	}

	return cmd
}

// harnessArgs is a harness command line split into our profile choice and
// what the harness is launched with.
type harnessArgs struct {
	profile     string
	chooseModel bool
	launch      harnesses.LaunchOptions
}

// parseHarnessArgs splits our leading flags from the harness's own arguments.
// A bare `--` ends our flags explicitly; otherwise the first token we do not
// recognise does. That keeps `requesty claude --model x -p "hi"` and
// `requesty claude -p "hi"` both working, and lets a harness flag that happens
// to share a name with ours (`codex --model`) still reach the harness when it
// appears later.
func parseHarnessArgs(args []string) (harnessArgs, error) {
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

		case arg == harnessProfileFlag || arg == harnessModelFlag || arg == harnessEffortFlag:
			if len(args) < 2 {
				return parsed, fmt.Errorf("%s requires a value", arg)
			}
			if err := parsed.set(arg, args[1]); err != nil {
				return parsed, err
			}
			args = args[2:]

		case strings.HasPrefix(arg, harnessProfileFlag+"=") || strings.HasPrefix(arg, harnessModelFlag+"=") || strings.HasPrefix(arg, harnessEffortFlag+"="):
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

	return parsed, nil
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
	case harnessEffortFlag:
		if !slices.Contains(harnesses.Efforts, value) {
			return fmt.Errorf("%s must be one of %s (got %q)", flag, strings.Join(harnesses.Efforts, ", "), value)
		}
		p.launch.Effort = value
	}

	return nil
}
