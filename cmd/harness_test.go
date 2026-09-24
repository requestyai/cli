package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/harnesses"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHarnessCommandsAreRegistered(t *testing.T) {
	command := newRootCommand(&environment{})

	for _, name := range []string{"claude", "codex", "opencode", "pi", "hermes"} {
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
			args: []string{"--profile", "eu", "--model", "anthropic/claude-fable-5", "--effort", "high", "--dangerously-skip-permissions", "-p", "hi"},
			want: harnessArgs{
				profile: "eu",
				launch: harnesses.LaunchOptions{
					Model: "anthropic/claude-fable-5",
					Args:  []string{"--effort", "high", "--dangerously-skip-permissions", "-p", "hi"},
				},
			},
		},
		{
			name: "equals form for every flag",
			args: []string{"--profile=eu", "--model=openai/gpt-5"},
			want: harnessArgs{profile: "eu", launch: harnesses.LaunchOptions{Model: "openai/gpt-5"}},
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
		{
			name: "fast model in both forms",
			args: []string{"--fast-model", "claude-haiku-4-5", "--model=claude-opus-4-8"},
			want: harnessArgs{launch: harnesses.LaunchOptions{Model: "claude-opus-4-8", FastModel: "claude-haiku-4-5"}},
		},
		{
			name: "choose fast model alongside a one-run model",
			args: []string{"--model", "claude-opus-4-8", "--choose-fast-model", "-p", "hi"},
			want: harnessArgs{chooseFastModel: true, launch: harnesses.LaunchOptions{Model: "claude-opus-4-8", Args: []string{"-p", "hi"}}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHarnessArgs(tt.args, claudeSpec)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestParseHarnessArgsErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
		spec harnessSpec
		want string
	}{
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
		{
			name: "fast model without value",
			args: []string{"--fast-model="},
			want: "--fast-model requires a value",
		},
		{
			name: "fast model and choose fast model together",
			args: []string{"--choose-fast-model", "--fast-model", "x"},
			want: "--fast-model and --choose-fast-model cannot be combined",
		},
		{
			name: "fast model for a harness without one",
			args: []string{"--fast-model", "x"},
			spec: codexSpec,
			want: "Codex has no separate model for background work; --fast-model does not apply",
		},
		{
			name: "choose fast model for a harness without one",
			args: []string{"--choose-fast-model"},
			spec: codexSpec,
			want: "Codex has no separate model for background work; --choose-fast-model does not apply",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			spec := tt.spec
			if spec.binary == "" {
				spec = claudeSpec
			}

			_, err := parseHarnessArgs(tt.args, spec)

			require.EqualError(t, err, tt.want)
		})
	}
}

func TestParseHarnessArgsHelp(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"--help"}, {"--model", "x", "--help"}} {
		_, err := parseHarnessArgs(args, claudeSpec)
		assert.ErrorIs(t, err, errHarnessHelp, "%v", args)
	}

	// After the harness's own flags begin, -h belongs to the harness.
	got, err := parseHarnessArgs([]string{"-p", "hi", "--help"}, claudeSpec)
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
	assert.Contains(t, output.String(), "--fast-model")
	assert.Contains(t, output.String(), "--choose-fast-model")
	assert.NotContains(t, output.String(), "reasoning-effort", "effort belongs to the harness's own flags")
	assert.Contains(t, output.String(), "passed to `claude`")
	assert.Contains(t, output.String(), "claude-sonnet-4-6 is used when the profile can route to it")
	assert.Contains(t, output.String(), "claude-haiku-4-5")
}

func TestHarnessWithoutFastModelHidesItsFlags(t *testing.T) {
	command := newRootCommand(&environment{store: oneProfile})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"codex", "--help"})

	require.NoError(t, command.Execute())

	assert.Contains(t, output.String(), "--choose-model")
	assert.NotContains(t, output.String(), "fast-model")
}

func TestHarnessWithoutFastModelRejectsItsFlags(t *testing.T) {
	for _, args := range [][]string{{"--fast-model", "x"}, {"--choose-fast-model"}} {
		command := newRootCommand(&environment{store: oneProfile})
		command.SetArgs(append([]string{"codex"}, args...))

		err := command.Execute()

		require.ErrorContains(t, err, "Codex has no separate model for background work", "%v", args)
		require.ErrorContains(t, err, args[0]+" does not apply", "%v", args)
	}
}

var (
	claudeSpec = harnessSpec{binary: "claude", displayName: "Claude Code", defaultModels: []string{"claude-sonnet-4-6"}, defaultFastModels: []string{"claude-haiku-4-5"}}
	codexSpec  = harnessSpec{binary: "codex", displayName: "Codex", defaultModels: []string{"gpt-5.5"}}
)

// routerServing is a router whose two model lists both hold ids. With none,
// hitting it fails the test: the caller expects no lookup.
func routerServing(t *testing.T, ids ...string) string {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(ids) == 0 {
			t.Errorf("unexpected request to %s", r.URL.Path)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		data := make([]map[string]string, 0, len(ids))
		for _, id := range ids {
			data = append(data, map[string]string{"id": id})
		}
		require.NoError(t, json.NewEncoder(w).Encode(map[string]any{"data": data}))
	}))
	t.Cleanup(server.Close)

	return server.URL
}

// modelsEnvironment is an environment with one profile on router, saving to
// a scratch home, plus a command to run against it and its stderr.
func modelsEnvironment(t *testing.T, router string, profile config.Config) (*environment, config.Config, *cobra.Command, *bytes.Buffer) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	profile.APIKey, profile.RouterBaseURL = "k", router
	env := &environment{store: config.Store{Current: "work", Profiles: map[string]config.Config{"work": profile}}}
	cfg, err := env.store.Resolve("")
	require.NoError(t, err)

	var stderr bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.SetErr(&stderr)

	return env, cfg, cmd, &stderr
}

func TestEnsureModelsUsesRoutableDefaultsAndRemembersThem(t *testing.T) {
	env, cfg, cmd, stderr := modelsEnvironment(t, routerServing(t, "claude-sonnet-4-6", "claude-haiku-4-5"), config.Config{})

	model, fast, err := env.ensureModels(cmd, cfg, claudeSpec, harnessArgs{})

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-6", model)
	assert.Equal(t, "claude-haiku-4-5", fast)
	assert.Contains(t, stderr.String(), "Using claude-sonnet-4-6 for Claude Code; change with --choose-model.")
	assert.Contains(t, stderr.String(), "Using claude-haiku-4-5 for Claude Code background work; change with --choose-fast-model.")
	saved, err := config.Load()
	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-6", saved.Profiles["work"].HarnessModels["claude"])
	assert.Equal(t, "claude-haiku-4-5", saved.Profiles["work"].HarnessFastModels["claude"])
}

func TestEnsureModelsTriesDefaultsInOrder(t *testing.T) {
	env, cfg, cmd, _ := modelsEnvironment(t, routerServing(t, "claude-sonnet-4-5", "claude-haiku-3-5"), config.Config{})
	spec := claudeSpec
	spec.defaultModels = []string{"claude-sonnet-4-6", "claude-sonnet-4-5"}
	spec.defaultFastModels = []string{"claude-haiku-4-5", "claude-haiku-3-5"}

	model, fast, err := env.ensureModels(cmd, cfg, spec, harnessArgs{})

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-5", model)
	assert.Equal(t, "claude-haiku-3-5", fast)
}

func TestEnsureModelsFallsBackToTheMainModelForBackgroundWork(t *testing.T) {
	env, cfg, cmd, stderr := modelsEnvironment(t, routerServing(t, "claude-sonnet-4-6"), config.Config{})

	model, fast, err := env.ensureModels(cmd, cfg, claudeSpec, harnessArgs{})

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-6", model)
	assert.Equal(t, "claude-sonnet-4-6", fast)
	assert.Contains(t, stderr.String(), "Using claude-sonnet-4-6 for Claude Code background work")
}

func TestEnsureModelsForgetsFastModelWhenMainModelIsNewlyPicked(t *testing.T) {
	env, cfg, cmd, _ := modelsEnvironment(t, routerServing(t, "claude-sonnet-4-6", "claude-haiku-4-5"), config.Config{
		HarnessFastModels: map[string]string{"claude": "claude-haiku-4-5@eu"},
	})

	model, fast, err := env.ensureModels(cmd, cfg, claudeSpec, harnessArgs{})

	require.NoError(t, err)
	assert.Equal(t, "claude-sonnet-4-6", model)
	assert.Equal(t, "claude-haiku-4-5", fast, "the remembered fast model is settled again next to the new main model")
	assert.Equal(t, "claude-haiku-4-5", env.store.Profiles["work"].HarnessFastModels["claude"])
}

func TestEnsureModelsUsesWhatIsRememberedWithoutLookingUp(t *testing.T) {
	env, cfg, cmd, stderr := modelsEnvironment(t, routerServing(t), config.Config{
		HarnessModels:     map[string]string{"claude": "claude-fable-5"},
		HarnessFastModels: map[string]string{"claude": "claude-haiku-4-5@eu"},
	})

	model, fast, err := env.ensureModels(cmd, cfg, claudeSpec, harnessArgs{})

	require.NoError(t, err)
	assert.Equal(t, "claude-fable-5", model)
	assert.Equal(t, "claude-haiku-4-5@eu", fast)
	assert.Empty(t, stderr.String())
	assert.NoFileExists(t, config.DisplayPath(), "nothing changed, so nothing is written")
}

func TestEnsureModelsFlagsAreForThisRunOnly(t *testing.T) {
	env, cfg, cmd, _ := modelsEnvironment(t, routerServing(t), config.Config{
		HarnessModels:     map[string]string{"claude": "claude-fable-5"},
		HarnessFastModels: map[string]string{"claude": "claude-haiku-4-5"},
	})
	parsed := harnessArgs{launch: harnesses.LaunchOptions{Model: "anthropic/claude-opus-4-1", FastModel: "anthropic/claude-haiku-4-5"}}

	model, fast, err := env.ensureModels(cmd, cfg, claudeSpec, parsed)

	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-opus-4-1", model)
	assert.Equal(t, "anthropic/claude-haiku-4-5", fast)
	assert.Equal(t, "claude-fable-5", env.store.Profiles["work"].HarnessModels["claude"])
	assert.NoFileExists(t, config.DisplayPath())
}

func TestEnsureModelsSettlesBackgroundWorkNextToAOneRunModel(t *testing.T) {
	env, cfg, cmd, _ := modelsEnvironment(t, routerServing(t, "claude-haiku-4-5"), config.Config{})
	parsed := harnessArgs{launch: harnesses.LaunchOptions{Model: "anthropic/claude-opus-4-1"}}

	model, fast, err := env.ensureModels(cmd, cfg, claudeSpec, parsed)

	require.NoError(t, err)
	assert.Equal(t, "anthropic/claude-opus-4-1", model)
	assert.Equal(t, "claude-haiku-4-5", fast)
	assert.Empty(t, env.store.Profiles["work"].HarnessModels, "a one-run model is not remembered")
	assert.Equal(t, "claude-haiku-4-5", env.store.Profiles["work"].HarnessFastModels["claude"])
}

func TestEnsureModelsWithoutAFastModel(t *testing.T) {
	env, cfg, cmd, stderr := modelsEnvironment(t, routerServing(t, "gpt-5.5"), config.Config{})

	model, fast, err := env.ensureModels(cmd, cfg, codexSpec, harnessArgs{})

	require.NoError(t, err)
	assert.Equal(t, "gpt-5.5", model)
	assert.Empty(t, fast)
	assert.NotContains(t, stderr.String(), "background work")
	assert.Empty(t, env.store.Profiles["work"].HarnessFastModels)
}

func TestHarnessProfileFlag(t *testing.T) {
	command := newRootCommand(&environment{store: twoProfiles})
	command.SetArgs([]string{"claude", "--profile", "sales"})

	err := command.Execute()

	require.ErrorContains(t, err, `profile "sales" not found`)
}
