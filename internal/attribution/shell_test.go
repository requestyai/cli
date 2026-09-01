package attribution

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallShellHookIsIdempotent(t *testing.T) {
	home := shellHome(t, "zsh")
	startupPath := filepath.Join(home, ".zshrc")
	require.NoError(t, os.WriteFile(startupPath, []byte("alias ll='ls -l'\n"), 0o644))

	set, err := New()
	require.NoError(t, err)

	hook, err := InstallShellHook(set)
	require.NoError(t, err)
	assert.Equal(t, startupPath, hook.StartupFilePath)
	assert.Equal(t, filepath.Join(home, ".requesty", "shell", "attribution.sh"), hook.HookPath)

	installed, err := ShellHookInstalled()
	require.NoError(t, err)
	assert.True(t, installed)

	after := readFile(t, startupPath)
	assert.Contains(t, after, "alias ll='ls -l'")
	assert.Contains(t, after, hook.HookPath)
	assert.Equal(t, 1, strings.Count(after, markerStart))

	// The startup file the user had is kept once, before the first change.
	assert.Equal(t, "alias ll='ls -l'\n", readFile(t, startupPath+".requesty.bak"))

	_, err = InstallShellHook(set)
	require.NoError(t, err)
	assert.Equal(t, after, readFile(t, startupPath))
}

func TestRemoveShellHookRestoresTheStartupFile(t *testing.T) {
	home := shellHome(t, "bash")
	startupPath := filepath.Join(home, ".bashrc")
	original := "export EDITOR=vim\n"
	require.NoError(t, os.WriteFile(startupPath, []byte(original), 0o644))

	set, err := New()
	require.NoError(t, err)
	hook, err := InstallShellHook(set)
	require.NoError(t, err)

	require.NoError(t, RemoveShellHook())

	assert.Equal(t, original, readFile(t, startupPath))
	_, err = os.Stat(hook.HookPath)
	assert.True(t, os.IsNotExist(err))

	installed, err := ShellHookInstalled()
	require.NoError(t, err)
	assert.False(t, installed)

	// Removing again is not an error, and leaves the file alone.
	require.NoError(t, RemoveShellHook())
	assert.Equal(t, original, readFile(t, startupPath))
}

func TestInstallShellHookCreatesAMissingStartupFile(t *testing.T) {
	home := shellHome(t, "fish")

	set, err := New()
	require.NoError(t, err)
	hook, err := InstallShellHook(set)
	require.NoError(t, err)

	assert.Equal(t, filepath.Join(home, ".config", "fish", "config.fish"), hook.StartupFilePath)
	assert.Contains(t, readFile(t, hook.StartupFilePath), "source")
	assert.Contains(t, readFile(t, hook.HookPath), "--on-event fish_prompt")
}

func TestInstallShellHookRejectsAnUnsupportedShell(t *testing.T) {
	shellHome(t, "tcsh")

	_, err := InstallShellHook(Set{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported shell")
}

// The hook is only useful if a shell that sources it really ends up with the
// dimensions exported, so run it.
func TestPosixHookExportsTheDimensions(t *testing.T) {
	home := shellHome(t, "bash")
	set, err := New()
	require.NoError(t, err)
	hook, err := InstallShellHook(set)
	require.NoError(t, err)

	repo := gitRepo(t)
	output, err := run(t, repo, "sh", "-c",
		". "+hook.HookPath+`; printf '%s|%s|%s' "$REQUESTY_REPO" "$REQUESTY_BRANCH" "$ANTHROPIC_CUSTOM_HEADERS"`)
	require.NoError(t, err)

	fields := strings.SplitN(output, "|", 3)
	require.Len(t, fields, 3)
	assert.Equal(t, "requestyai/cli", fields[0])
	assert.Equal(t, "main", fields[1])
	assert.Equal(t, strings.Join([]string{
		"X-Requesty-Repo: requestyai/cli",
		"X-Requesty-Branch: main",
		"X-Requesty-User: " + userDimension(t, set).Value,
	}, "\n"), fields[2])

	// Nothing in the hook depends on the home directory it was written from.
	assert.NotContains(t, readFile(t, hook.HookPath), filepath.Join(home, ".zshrc"))
}

func TestFishHookExportsTheDimensions(t *testing.T) {
	if _, err := exec.LookPath("fish"); err != nil {
		t.Skip("fish is not installed")
	}

	shellHome(t, "fish")
	set, err := New()
	require.NoError(t, err)
	hook, err := InstallShellHook(set)
	require.NoError(t, err)

	output, err := run(t, gitRepo(t), "fish", "--no-config", "--command",
		"source "+hook.HookPath+
			`; printf '%s|%s' "$REQUESTY_REPO" "$ANTHROPIC_CUSTOM_HEADERS"`)
	require.NoError(t, err)

	fields := strings.SplitN(output, "|", 2)
	require.Len(t, fields, 2)
	assert.Equal(t, "requestyai/cli", fields[0])
	assert.Contains(t, fields[1], "X-Requesty-Branch: main")
}

func TestWithoutBlockLeavesTheRestOfTheFileAlone(t *testing.T) {
	content := "export EDITOR=vim\n\n" + markerStart + "\nsource hook\n" + markerEnd + "\n\nalias g=git\n"

	assert.Equal(t, "export EDITOR=vim\n\nalias g=git\n", withoutBlock(content))
}

// shellHome points the CLI at a throwaway home directory and shell, so
// installing the hook cannot touch the startup file of whoever runs the tests.
func shellHome(t *testing.T, shell string) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("SHELL", filepath.Join("/opt/homebrew/bin", shell))

	return home
}

func userDimension(t *testing.T, set Set) Dimension {
	t.Helper()

	for _, dimension := range set {
		if dimension.Header == headerUser {
			return dimension
		}
	}
	t.Fatal("the set has no user dimension")

	return Dimension{}
}

func readFile(t *testing.T, path string) string {
	t.Helper()

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	return string(content)
}
