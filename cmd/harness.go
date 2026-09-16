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
	harnessModelFlag  = "--model"
	harnessEffortFlag = "--reasoning-effort"
)

// errHarnessHelp signals that the leading flags asked for help.
var errHarnessHelp = errors.New("help requested")

// newHarnessCommands returns the `requesty <harness>` commands, each of which
// starts a harness with Requesty injected for that run.
func newHarnessCommands(env environment) []*cobra.Command {
	cfg := withRouterDefault(env.config)

	return []*cobra.Command{
		newHarnessCommand(env, "claude", "Claude Code", func() (harnesses.Harness, error) {
			dir, err := harnesses.DefaultConfigDirClaudeCode()
			if err != nil {
				return nil, err
			}
			return harnesses.NewClaudeHarness(cfg, dir), nil
		}),
		newHarnessCommand(env, "codex", "Codex", func() (harnesses.Harness, error) {
			dir, err := harnesses.DefaultConfigDirCodex()
			if err != nil {
				return nil, err
			}
			return harnesses.NewCodexHarness(cfg, dir), nil
		}),
	}
}

// newHarnessCommand builds `requesty <binary>`. The harness is constructed
// lazily so a machine without a resolvable home directory still gets a help
// page and a clear error, rather than a missing command.
func newHarnessCommand(env environment, binary, displayName string, newHarness func() (harnesses.Harness, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s [--model <id>] [--reasoning-effort <level>] [-- ] [%s args...]", binary, binary),
		Short: fmt.Sprintf("Launch %s through Requesty", displayName),
		Long: fmt.Sprintf("Launch %s with its traffic routed through Requesty for this run only.\n\n", displayName) +
			"Flags:\n" +
			fmt.Sprintf("  %-28s %s\n", harnessModelFlag+" <id>", "Model to use for this run (any Requesty model id)") +
			fmt.Sprintf("  %-28s %s\n", harnessEffortFlag+" <level>", "Reasoning effort: "+strings.Join(harnesses.Efforts, ", ")) +
			fmt.Sprintf("  %-28s %s", "-h, --help", "Show this help"),
		// We take over parsing so harness flags are never interpreted as ours.
		DisableFlagParsing:    true,
		DisableFlagsInUseLine: true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts, err := parseHarnessArgs(args)
			if errors.Is(err, errHarnessHelp) {
				return cmd.Help()
			}
			if err != nil {
				return err
			}

			if env.config.APIKey == "" {
				return fmt.Errorf("no Requesty API key configured; run `requesty` to set one up")
			}

			harness, err := newHarness()
			if err != nil {
				return fmt.Errorf("failed to set up %s: %w", displayName, err)
			}

			launch := env.launchHarness
			if launch == nil {
				launch = harnesses.Harness.Launch
			}

			return launch(harness, opts)
		},
	}

	return cmd
}

// parseHarnessArgs splits our leading flags from the harness's own arguments.
// A bare `--` ends our flags explicitly; otherwise the first token we do not
// recognise does. That keeps `requesty claude --model x -p "hi"` and
// `requesty claude -p "hi"` both working, and lets a harness flag that happens
// to share a name with ours (`codex --model`) still reach the harness when it
// appears later.
func parseHarnessArgs(args []string) (harnesses.LaunchOptions, error) {
	var opts harnesses.LaunchOptions

	for len(args) > 0 {
		arg := args[0]

		switch {
		case arg == "--":
			opts.Args = args[1:]
			return opts, nil

		case arg == "-h" || arg == "--help":
			return opts, errHarnessHelp

		case arg == harnessModelFlag || arg == harnessEffortFlag:
			if len(args) < 2 {
				return opts, fmt.Errorf("%s requires a value", arg)
			}
			if err := setHarnessFlag(&opts, arg, args[1]); err != nil {
				return opts, err
			}
			args = args[2:]

		case strings.HasPrefix(arg, harnessModelFlag+"=") || strings.HasPrefix(arg, harnessEffortFlag+"="):
			flag, value, _ := strings.Cut(arg, "=")
			if err := setHarnessFlag(&opts, flag, value); err != nil {
				return opts, err
			}
			args = args[1:]

		default:
			opts.Args = args
			return opts, nil
		}
	}

	return opts, nil
}

func setHarnessFlag(opts *harnesses.LaunchOptions, flag, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s requires a value", flag)
	}

	switch flag {
	case harnessModelFlag:
		opts.Model = value
	case harnessEffortFlag:
		if !slices.Contains(harnesses.Efforts, value) {
			return fmt.Errorf("%s must be one of %s (got %q)", flag, strings.Join(harnesses.Efforts, ", "), value)
		}
		opts.Effort = value
	}

	return nil
}

// withRouterDefault fills in the production router when the config predates
// the field, so a launch never hands a harness an empty base URL.
func withRouterDefault(cfg config.Config) config.Config {
	if cfg.RouterBaseURL == "" {
		cfg.RouterBaseURL = config.DefaultRouterBaseURL
	}

	return cfg
}
