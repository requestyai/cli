//go:build !windows

package harnesses

import "syscall"

// execProcess replaces the current process with the harness, so the harness
// owns the terminal directly and requesty leaves no wrapper behind. It is a
// variable so tests can capture the launch instead of performing it.
var execProcess = func(path string, argv []string, env []string) error {
	return syscall.Exec(path, argv, env)
}
