package harnesses

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
)

// Effort levels understood by Launch. Each harness translates them to its own
// vocabulary and rejects the ones it cannot express.
const (
	EffortMinimal = "minimal"
	EffortLow     = "low"
	EffortMedium  = "medium"
	EffortHigh    = "high"
	EffortXHigh   = "xhigh"
	EffortMax     = "max"
)

// Efforts lists every level Launch accepts, weakest first.
var Efforts = []string{EffortMinimal, EffortLow, EffortMedium, EffortHigh, EffortXHigh, EffortMax}

// mapEffort translates a Launch effort level through a harness's own table.
func mapEffort(harness string, table map[string]string, effort string) (string, error) {
	if effort == "" {
		return "", nil
	}

	if native, ok := table[effort]; ok {
		return native, nil
	}

	accepted := make([]string, 0, len(table))
	for level := range table {
		accepted = append(accepted, level)
	}
	sort.Slice(accepted, func(i, j int) bool {
		return slices.Index(Efforts, accepted[i]) < slices.Index(Efforts, accepted[j])
	})

	return "", fmt.Errorf("%s does not support reasoning effort %q (accepted: %s)",
		harness, effort, strings.Join(accepted, ", "))
}

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
