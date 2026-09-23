package cmd

import (
	"bytes"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessCommandsAreRegistered(t *testing.T) {
	command := newRootCommand(&environment{})

	for _, name := range []string{"claude", "codex"} {
		sub, _, err := command.Find([]string{name})
		require.NoError(t, err)
		assert.Equal(t, name, sub.Name())
		assert.True(t, sub.DisableFlagParsing, "%s must not interpret harness flags", name)
	}
}

func TestParseHarnessArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want harnessArgs
	}{
		{
			name: "no arguments",
			args: nil,
			want: harnessArgs{},
		},
		{
			name: "leading flags then passthrough",
			args: []string{"--profile", "eu", "--model", "anthropic/claude-fable-5", "--reasoning-effort=high", "--dangerously-skip-permissions", "-p", "hi"},
			want: harnessArgs{
				profile: "eu",
				launch: harnesses.LaunchOptions{
					Model:  "anthropic/claude-fable-5",
					Effort: harnesses.EffortHigh,
					Args:   []string{"--dangerously-skip-permissions", "-p", "hi"},
				},
			},
		},
		{
			name: "equals form for every flag",
			args: []string{"--profile=eu", "--model=openai/gpt-5", "--reasoning-effort=low"},
			want: harnessArgs{profile: "eu", launch: harnesses.LaunchOptions{Model: "openai/gpt-5", Effort: harnesses.EffortLow}},
		},
		{
			// The second --model belongs to the harness because a foreign flag came first.
			name: "stops at first foreign argument",
			args: []string{"--full-auto", "--model", "theirs"},
			want: harnessArgs{launch: harnesses.LaunchOptions{Args: []string{"--full-auto", "--model", "theirs"}}},
		},
		{
			name: "double dash ends our flags",
			args: []string{"--model", "openai/gpt-5", "--", "--model", "theirs"},
			want: harnessArgs{launch: harnesses.LaunchOptions{Model: "openai/gpt-5", Args: []string{"--model", "theirs"}}},
		},
		{
			name: "bare double dash passes nothing",
			args: []string{"--"},
			want: harnessArgs{launch: harnesses.LaunchOptions{Args: []string{}}},
		},
		{
			name: "choose model takes no value",
			args: []string{"--choose-model", "--profile", "eu", "-p", "hi"},
			want: harnessArgs{profile: "eu", chooseModel: true, launch: harnesses.LaunchOptions{Args: []string{"-p", "hi"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHarnessArgs(tt.args)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseHarnessArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "unknown effort",
			args: []string{"--reasoning-effort", "turbo"},
			want: `--reasoning-effort must be one of minimal, low, medium, high, xhigh, max (got "turbo")`,
		},
		{
			name: "model without value",
			args: []string{"--model"},
			want: "--model requires a value",
		},
		{
			name: "profile without value",
			args: []string{"--profile"},
			want: "--profile requires a value",
		},
		{
			name: "effort without value",
			args: []string{"--reasoning-effort"},
			want: "--reasoning-effort requires a value",
		},
		{
			name: "empty equals value",
			args: []string{"--model="},
			want: "--model requires a value",
		},
		{
			name: "blank value",
			args: []string{"--model", "  "},
			want: "--model requires a value",
		},
		{
			name: "model and choose model together",
			args: []string{"--model", "x", "--choose-model", "--", "-p", "hi"},
			want: "--model and --choose-model cannot be combined",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parseHarnessArgs(tt.args)

			require.EqualError(t, err, tt.want)
		})
	}
}

func TestParseHarnessArgsHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"--model", "x", "--help"}} {
		_, err := parseHarnessArgs(args)
		assert.ErrorIs(t, err, errHarnessHelp, "%v", args)
	}

	// After the harness's own flags begin, -h belongs to the harness.
	got, err := parseHarnessArgs([]string{"-p", "hi", "--help"})
	require.NoError(t, err)
	assert.Equal(t, []string{"-p", "hi", "--help"}, got.launch.Args)
}

func TestHarnessCommandHelpShowsOurFlags(t *testing.T) {
	command := newRootCommand(&environment{store: oneProfile})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"claude", "--help"})

	require.NoError(t, command.Execute())

	assert.Contains(t, output.String(), "--profile")
	assert.Contains(t, output.String(), "--choose-model")
	assert.Contains(t, output.String(), "--reasoning-effort")
	assert.Contains(t, output.String(), "passed to `claude`")
	assert.Contains(t, output.String(), "suggesting claude-sonnet-4-6")
}

func TestModelToLaunch(t *testing.T) {
	spec := harnessSpec{binary: "claude", displayName: "Claude Code", defaultModel: "claude-sonnet-4-6"}
	var fresh config.Config
	var picked config.Config
	picked.SetHarnessModel("claude", "claude-fable-5")

	tests := []struct {
		name      string
		cfg       config.Config
		parsed    harnessArgs
		wantModel string
		wantAsk   bool
	}{
		{
			name:      "flag wins over the saved model",
			cfg:       picked,
			parsed:    harnessArgs{launch: harnesses.LaunchOptions{Model: "anthropic/claude-opus-4-1"}},
			wantModel: "anthropic/claude-opus-4-1",
		},
		{
			name:      "saved model is used without asking",
			cfg:       picked,
			wantModel: "claude-fable-5",
		},
		{
			name:      "nothing saved asks, suggesting the harness default",
			cfg:       fresh,
			wantModel: "claude-sonnet-4-6",
			wantAsk:   true,
		},
		{
			name:      "choose model asks again, suggesting the saved model",
			cfg:       picked,
			parsed:    harnessArgs{chooseModel: true},
			wantModel: "claude-fable-5",
			wantAsk:   true,
		},
		{
			name:      "another harness's choice does not count",
			cfg:       func() config.Config { var c config.Config; c.SetHarnessModel("codex", "gpt-5.5"); return c }(),
			wantModel: "claude-sonnet-4-6",
			wantAsk:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, ask := modelToLaunch(tt.cfg, spec, tt.parsed)

			assert.Equal(t, tt.wantModel, model)
			assert.Equal(t, tt.wantAsk, ask)
		})
	}
}

func TestHarnessProfileFlag(t *testing.T) {
	command := newRootCommand(&environment{store: twoProfiles})
	command.SetArgs([]string{"claude", "--profile", "sales"})

	err := command.Execute()

	require.ErrorContains(t, err, `profile "sales" not found`)
}

func TestHarnessProfileFromEnvironment(t *testing.T) {
	t.Setenv(profileEnv, "sales")
	command := newRootCommand(&environment{store: twoProfiles})
	command.SetArgs([]string{"claude"})

	err := command.Execute()

	require.ErrorContains(t, err, `profile "sales" not found`)
}
