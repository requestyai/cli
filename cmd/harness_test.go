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
	command := newRootCommand(environment{})

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
		want harnesses.LaunchOptions
	}{
		{
			name: "no arguments",
			args: nil,
			want: harnesses.LaunchOptions{},
		},
		{
			name: "leading flags then passthrough",
			args: []string{"--model", "anthropic/claude-fable-5", "--reasoning-effort=high", "--dangerously-skip-permissions", "-p", "hi"},
			want: harnesses.LaunchOptions{
				Model:  "anthropic/claude-fable-5",
				Effort: harnesses.EffortHigh,
				Args:   []string{"--dangerously-skip-permissions", "-p", "hi"},
			},
		},
		{
			name: "equals form for both flags",
			args: []string{"--model=openai/gpt-5", "--reasoning-effort=low"},
			want: harnesses.LaunchOptions{Model: "openai/gpt-5", Effort: harnesses.EffortLow},
		},
		{
			// The second --model belongs to the harness because a foreign flag came first.
			name: "stops at first foreign argument",
			args: []string{"--full-auto", "--model", "theirs"},
			want: harnesses.LaunchOptions{Args: []string{"--full-auto", "--model", "theirs"}},
		},
		{
			name: "double dash ends our flags",
			args: []string{"--model", "openai/gpt-5", "--", "--model", "theirs"},
			want: harnesses.LaunchOptions{Model: "openai/gpt-5", Args: []string{"--model", "theirs"}},
		},
		{
			name: "bare double dash passes nothing",
			args: []string{"--"},
			want: harnesses.LaunchOptions{Args: []string{}},
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
	assert.Equal(t, []string{"-p", "hi", "--help"}, got.Args)
}

func TestHarnessCommandHelpShowsOurFlags(t *testing.T) {
	command := newRootCommand(environment{config: config.Config{APIKey: "my-api-key"}})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"claude", "--help"})

	require.NoError(t, command.Execute())

	assert.Contains(t, output.String(), "--reasoning-effort")
	assert.Contains(t, output.String(), "passed to `claude`")
}
