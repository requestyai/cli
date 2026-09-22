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
	harnessProfileFlag = "--" + profileFlag
	harnessModelFlag   = "--model"
	harnessEffortFlag  = "--reasoning-effort"
)

// errHarnessHelp signals that the leading flags asked for help.
var errHarnessHelp = errors.New("help requested")

// newHarnessCommands returns the `requesty <harness>` commands, each of which
// starts a harness with Requesty injected for that run.
func newHarnessCommands(env *environment) []*cobra.Command {
	return []*cobra.Command{
		newHarnessCommand(env, "claude", "Claude Code", func(cfg config.Config) (harnesses.Harness, error) {
			dir, err := harnesses.DefaultConfigDirClaudeCode()
			if err != nil {
				return nil, err
			}
			return harnesses.NewClaudeHarness(cfg, dir), nil
		}),
		newHarnessCommand(env, "codex", "Codex", func(cfg config.Config) (harnesses.Harness, error) {
			dir, err := harnesses.DefaultConfigDirCodex()
			if err != nil {
				return nil, err
			}
			return harnesses.NewCodexHarness(cfg, dir), nil
		}),
	}
}

// newHarnessCommand builds `requesty <binary>`. The harness is constructed
// lazily, from the profile as it stands after any onboarding, so a machine
// without a resolvable home directory still gets a help page and a clear
// error, rather than a missing command.
func newHarnessCommand(env *environment, binary, displayName string, newHarness func(config.Config) (harnesses.Harness, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [--profile <name>] [--model <id>] [--reasoning-effort <level>] [-- ] [%s args...]", binary, binary),
		Short: fmt.Sprintf("Launch %s through Requesty", displayName),
		Long: fmt.Sprintf("Launch %s with its traffic routed through Requesty for this run only.\n\n", displayName) +
			"Flags:\n" +
			fmt.Sprintf("  %-28s %s\n", harnessProfileFlag+" <name>", "Saved profile to run as (also "+profileEnv+")") +
			fmt.Sprintf("  %-28s %s\n", harnessModelFlag+" <id>", "Model to use for this run (any Requesty model id)") +
			fmt.Sprintf("  %-28s %s\n", harnessEffortFlag+" <level>", "Reasoning effort: "+strings.Join(harnesses.Efforts, ", ")) +
			fmt.Sprintf("  %-28s %s\n\n", "-h, --help", "Show this help") +
			fmt.Sprintf("Anything else, or everything after `--`, is passed to `%s` untouched.\n\n", binary),
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

			cfg, err := env.ensureProfile(cmd, parsed.profile, displayName)
			if err != nil {
				return err
			}

			harness, err := newHarness(cfg)
			if err != nil {
				return fmt.Errorf("failed to set up %s: %w", displayName, err)
			}

			return harness.Launch(parsed.launch)
		},
	}

	return cmd
}

// harnessArgs is a harness command line split into our profile choice and
// what the harness is launched with.
type harnessArgs struct {
	profile string
	launch  harnesses.LaunchOptions
}

// parseHarnessArgs splits our leading flags from the harness's own arguments.
// A bare `--` ends our flags explicitly; otherwise the first token we do not
// recognise does. That keeps `requesty claude --model x -p "hi"` and
// `requesty claude -p "hi"` both working, and lets a harness flag that happens
// to share a name with ours (`codex --model`) still reach the harness when it
// appears later.
func parseHarnessArgs(args []string) (harnessArgs, error) {
	var parsed harnessArgs

	for len(args) > 0 {
		arg := args[0]

		switch {
		case arg == "--":
			parsed.launch.Args = args[1:]
			return parsed, nil

		case arg == "-h" || arg == "--help":
			return parsed, errHarnessHelp

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
			return parsed, nil
		}
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
