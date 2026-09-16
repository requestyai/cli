package cmd

import (
	"bytes"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordedLaunch struct {
	harness harnesses.Harness
	opts    harnesses.LaunchOptions
	called  bool
}

func recordingEnvironment(cfg config.Config) (environment, *recordedLaunch) {
	recorded := &recordedLaunch{}
	env := environment{
		config: cfg,
		launchHarness: func(harness harnesses.Harness, opts harnesses.LaunchOptions) error {
			recorded.harness, recorded.opts, recorded.called = harness, opts, true
			return nil
		},
	}

	return env, recorded
}

func TestHarnessCommandsAreRegistered(t *testing.T) {
	command := newRootCommand(environment{})

	for _, name := range []string{"claude", "codex"} {
		sub, _, err := command.Find([]string{name})
		require.NoError(t, err)
		assert.Equal(t, name, sub.Name())
		assert.True(t, sub.DisableFlagParsing, "%s must not interpret harness flags", name)
	}
}

func TestHarnessCommandPassesLeadingFlagsAndPassthrough(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{APIKey: "my-api-key"})
	command := newRootCommand(env)
	command.SetArgs([]string{"claude", "--model", "anthropic/claude-fable-5", "--reasoning-effort=high", "--dangerously-skip-permissions", "-p", "hi"})

	require.NoError(t, command.Execute())

	require.True(t, recorded.called)
	assert.Equal(t, "Claude Code", recorded.harness.Name())
	assert.Equal(t, "anthropic/claude-fable-5", recorded.opts.Model)
	assert.Equal(t, harnesses.EffortHigh, recorded.opts.Effort)
	assert.Equal(t, []string{"--dangerously-skip-permissions", "-p", "hi"}, recorded.opts.Args)
}

func TestHarnessCommandStopsAtFirstForeignArgument(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{APIKey: "my-api-key"})
	command := newRootCommand(env)
	// The second --model belongs to codex because a codex flag came first.
	command.SetArgs([]string{"codex", "--full-auto", "--model", "theirs"})

	require.NoError(t, command.Execute())

	assert.Empty(t, recorded.opts.Model)
	assert.Equal(t, []string{"--full-auto", "--model", "theirs"}, recorded.opts.Args)
}

func TestHarnessCommandDoubleDashEndsOurFlags(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{APIKey: "my-api-key"})
	command := newRootCommand(env)
	command.SetArgs([]string{"codex", "--model", "openai/gpt-5", "--", "--model", "theirs"})

	require.NoError(t, command.Execute())

	assert.Equal(t, "openai/gpt-5", recorded.opts.Model)
	assert.Equal(t, []string{"--model", "theirs"}, recorded.opts.Args)
}

func TestHarnessCommandRejectsUnknownEffort(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{APIKey: "my-api-key"})
	command := newRootCommand(env)
	command.SetArgs([]string{"claude", "--reasoning-effort", "turbo"})

	err := command.Execute()

	require.EqualError(t, err, `--reasoning-effort must be one of minimal, low, medium, high, xhigh, max (got "turbo")`)
	assert.False(t, recorded.called)
}

func TestHarnessCommandRejectsFlagWithoutValue(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{APIKey: "my-api-key"})
	command := newRootCommand(env)
	command.SetArgs([]string{"claude", "--model"})

	require.EqualError(t, command.Execute(), "--model requires a value")
	assert.False(t, recorded.called)
}

func TestHarnessCommandRequiresAPIKey(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{})
	command := newRootCommand(env)
	command.SetArgs([]string{"codex"})

	require.EqualError(t, command.Execute(), "no Requesty API key configured; run `requesty` to set one up")
	assert.False(t, recorded.called)
}

func TestHarnessCommandHelpDoesNotLaunch(t *testing.T) {
	env, recorded := recordingEnvironment(config.Config{APIKey: "my-api-key"})
	command := newRootCommand(env)
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"claude", "--help"})

	require.NoError(t, command.Execute())

	assert.False(t, recorded.called)
	assert.Contains(t, output.String(), "--reasoning-effort")
	assert.Contains(t, output.String(), "passed to `claude`")
}

func TestWithRouterDefaultFillsMissingURL(t *testing.T) {
	assert.Equal(t, config.DefaultRouterBaseURL, withRouterDefault(config.Config{APIKey: "k"}).RouterBaseURL)
	assert.Equal(t, "https://router.example.test", withRouterDefault(config.Config{RouterBaseURL: "https://router.example.test"}).RouterBaseURL)
}
