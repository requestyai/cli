//go:build windows

package harnesses

import (
	"errors"
	"os"
	"os/exec"
)

// execProcess runs the harness as a child with our terminal and exits with
// its status once it finishes. Windows has no exec(2), so this is the closest
// equivalent. It is a variable so tests can capture the launch instead.
var execProcess = func(path string, argv []string, env []string) error {
	cmd := exec.Command(path, argv[1:]...)
	cmd.Env = env
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		return err
	}

	os.Exit(0)
	return nil
}
