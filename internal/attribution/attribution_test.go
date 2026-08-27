package attribution

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSetSpellsDimensionsPerHarness(t *testing.T) {
	set := Set{
		{Header: headerRepo, Env: envRepo, Shell: repoShell},
		{Header: headerUser, Value: "ada"},
	}

	assert.Equal(t, map[string]string{
		"X-Title":         "Pi",
		"X-Requesty-User": "ada",
		"X-Requesty-Repo": "!" + repoShell,
	}, set.All(map[string]string{"X-Title": "Pi"}, ShellCommand))

	assert.Equal(t, map[string]string{
		"X-Requesty-Repo": "{env:REQUESTY_REPO}",
		"X-Requesty-User": "ada",
	}, set.All(nil, EnvPlaceholder))

	assert.Equal(t, map[string]string{
		"X-Requesty-User": "ada",
	}, set.Static(nil))

	assert.Equal(t, map[string]string{
		"X-Requesty-Repo": "REQUESTY_REPO",
	}, set.Dynamic(EnvName))
}

func TestEmptySetWritesNoHeaders(t *testing.T) {
	var set Set

	assert.Nil(t, set.Static(nil))
	assert.Nil(t, set.Dynamic(EnvName))
	assert.Nil(t, set.All(nil, ShellCommand))
	assert.Equal(t, map[string]string{"X-Title": "Pi"},
		set.All(map[string]string{"X-Title": "Pi"}, ShellCommand))
}

func TestNewResolvesTheUserAndDefersTheRest(t *testing.T) {
	set, err := New()
	require.NoError(t, err)

	byHeader := make(map[string]Dimension, len(set))
	for _, dimension := range set {
		byHeader[dimension.Header] = dimension
	}

	assert.False(t, byHeader[headerUser].Dynamic())
	assert.NotEmpty(t, byHeader[headerUser].Value)
	assert.True(t, byHeader[headerRepo].Dynamic())
	assert.True(t, byHeader[headerBranch].Dynamic())
}

func TestSanitizeKeepsHeaderValuesSafe(t *testing.T) {
	assert.Equal(t, "ada.lovelace_1", sanitize("ada.lovelace_1"))
	assert.Equal(t, "CORP-ada", sanitize(`CORP\ada`))
	assert.Equal(t, "ada-branch--x", sanitize("ada\nbranch: x"))
	assert.Equal(t, unknown, sanitize(""))
}

// The fish hook passes the shell commands to `sh -c '…'`, which a single quote
// in one of them would cut short.
func TestShellCommandsCarryNoSingleQuotes(t *testing.T) {
	for _, command := range []string{repoShell, branchShell} {
		assert.NotContains(t, command, "'")
	}
}

// Pi refuses to start when a header value comes back empty, so both commands
// have to print something and exit cleanly wherever they run.
func TestShellCommandsAlwaysPrintAValue(t *testing.T) {
	for name, dir := range map[string]string{
		"outside a repository": t.TempDir(),
		"inside a repository":  gitRepo(t),
	} {
		t.Run(name, func(t *testing.T) {
			for _, command := range []string{repoShell, branchShell} {
				output, err := run(t, dir, "sh", "-c", command)
				require.NoError(t, err)
				assert.NotEmpty(t, output)
			}
		})
	}
}

func TestShellCommandsReadTheRepositoryAndBranch(t *testing.T) {
	dir := gitRepo(t)

	repo, err := run(t, dir, "sh", "-c", repoShell)
	require.NoError(t, err)
	assert.Equal(t, "requestyai/cli", repo)

	branch, err := run(t, dir, "sh", "-c", branchShell)
	require.NoError(t, err)
	assert.Equal(t, "main", branch)
}

func TestShellCommandsFallBackOnADetachedHead(t *testing.T) {
	dir := gitRepo(t)
	_, err := run(t, dir, "git", "checkout", "--detach")
	require.NoError(t, err)

	branch, err := run(t, dir, "sh", "-c", branchShell)
	require.NoError(t, err)
	assert.Equal(t, unknown, branch)
}

func TestRepoShellTrimsEveryRemoteForm(t *testing.T) {
	remotes := map[string]string{
		"https://github.com/requestyai/cli.git":       "requestyai/cli",
		"https://token@github.com/requestyai/cli.git": "requestyai/cli",
		"git@github.com:requestyai/cli.git":           "requestyai/cli",
		"ssh://git@github.com/requestyai/cli.git":     "requestyai/cli",
		"https://gitlab.com/requestyai/team/cli":      "requestyai/team/cli",
	}

	for remote, expected := range remotes {
		t.Run(remote, func(t *testing.T) {
			dir := gitRepo(t)
			_, err := run(t, dir, "git", "remote", "set-url", "origin", remote)
			require.NoError(t, err)

			repo, err := run(t, dir, "sh", "-c", repoShell)
			require.NoError(t, err)
			assert.Equal(t, expected, repo)
		})
	}
}

// gitRepo creates a repository on branch main with an origin remote and one
// commit, so HEAD points at a branch the shell commands can read.
func gitRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	for _, args := range [][]string{
		{"git", "init", "--initial-branch", "main"},
		{"git", "remote", "add", "origin", "https://github.com/requestyai/cli.git"},
		{"git", "-c", "user.email=cli@requesty.ai", "-c", "user.name=CLI",
			"commit", "--allow-empty", "--message", "init"},
	} {
		_, err := run(t, dir, args[0], args[1:]...)
		require.NoError(t, err)
	}

	return dir
}

func run(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()

	command := exec.Command(name, args...)
	command.Dir = dir
	output, err := command.Output()

	return strings.TrimSpace(string(output)), err
}
