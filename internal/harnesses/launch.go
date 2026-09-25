package harnesses

import (
	"fmt"
	"os"
	"slices"
	"strings"
)

// launchNotImplemented is what harnesses without a launcher say instead of
// doing anything.
func launchNotImplemented(name string) error {
	_, _ = fmt.Fprintf(os.Stderr, "Launching %s through Requesty is not implemented yet.\n", name)
	return nil
}

// requestyExecutable is the command a harness should shell out to for
// credentials. It prefers our own absolute path so the launch does not depend
// on `requesty` being on the harness's PATH.
func requestyExecutable() string {
	if path, err := os.Executable(); err == nil && path != "" {
		return path
	}

	return "requesty"
}

// parseEnvironmentVariables turns the "KEY=value" strings os.Environ produces into a map,
// so a harness can set or delete variables by name. nil means the current
// process environment.
func parseEnvironmentVariables(env []string) map[string]string {
	if env == nil {
		env = os.Environ()
	}

	parsed := make(map[string]string, len(env))
	for _, entry := range env {
		key, value, _ := strings.Cut(entry, "=")
		parsed[key] = value
	}

	return parsed
}

// formatEnvironmentVariables is the inverse of parseEnvironmentVariables: it returns the "KEY=value"
// strings exec expects, sorted so the result is deterministic.
func formatEnvironmentVariables(env map[string]string) []string {
	formatted := make([]string, 0, len(env))
	for key, value := range env {
		formatted = append(formatted, key+"="+value)
	}
	slices.Sort(formatted)

	return formatted
}
