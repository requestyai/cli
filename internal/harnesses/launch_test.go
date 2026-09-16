package harnesses

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// capturedLaunch is what a harness's Launch handed to execProcess.
type capturedLaunch struct {
	path string
	argv []string
	env  []string
}

// captureExec swaps the process replacement for a recorder for one test.
func captureExec(t *testing.T) *capturedLaunch {
	t.Helper()

	captured := &capturedLaunch{}
	previous := execProcess
	execProcess = func(path string, argv []string, env []string) error {
		captured.path, captured.argv, captured.env = path, argv, env
		return nil
	}
	t.Cleanup(func() { execProcess = previous })

	return captured
}

// fakeBinary drops an executable named name into a temp dir that leads PATH
// and returns its path.
func fakeBinary(t *testing.T, name string) string {
	t.Helper()

	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	path := filepath.Join(dir, name)
	require.NoError(t, os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return path
}

func TestUnsupportedHarnessesLaunchAsNoOp(t *testing.T) {
	captured := captureExec(t)
	cfg := config.Config{RouterBaseURL: "https://router.requesty.ai", APIKey: "my-api-key"}

	for _, harness := range []Harness{
		NewOpenCodeHarness(cfg, t.TempDir()),
		NewPiHarness(cfg, t.TempDir()),
		NewHermesHarness(cfg, t.TempDir()),
		NewDeepSeekHarness(cfg, t.TempDir()),
	} {
		require.NoError(t, harness.Launch(LaunchOptions{}), harness.Name())
	}
	assert.Empty(t, captured.path, "nothing should be launched")
}

func TestMapEffortListsAcceptedLevelsInOrder(t *testing.T) {
	_, err := mapEffort("Codex", codexEfforts, EffortMax)

	require.EqualError(t, err, `Codex does not support reasoning effort "max" (accepted: minimal, low, medium, high, xhigh)`)
}

func TestMapEffortEmptyMeansHarnessDefault(t *testing.T) {
	native, err := mapEffort("Codex", codexEfforts, "")

	require.NoError(t, err)
	assert.Empty(t, native)
}

func TestEnvironRoundTripsAndSorts(t *testing.T) {
	env := parseEnvironmentVariables([]string{"C=3", "A=1", "B=2", "EQUALS=a=b"})
	env["B"] = "two"
	delete(env, "A")
	env["D"] = "4"

	assert.Equal(t, []string{"B=two", "C=3", "D=4", "EQUALS=a=b"}, formatEnvironmentVariables(env))
}

func TestParseEnvironNilMeansProcessEnvironment(t *testing.T) {
	t.Setenv("REQUESTY_TEST_MARKER", "present")

	assert.Equal(t, "present", parseEnvironmentVariables(nil)["REQUESTY_TEST_MARKER"])
}
