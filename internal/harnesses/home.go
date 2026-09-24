package harnesses

import (
	"fmt"
	"os"
	"path/filepath"
)

// configDirInHome joins elements onto the current user's home directory.
func configDirInHome(elements ...string) (string, error) {
	homePath, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home path: %w", err)
	}

	return filepath.Join(append([]string{homePath}, elements...)...), nil
}

// requestyDirInHome joins elements onto our own ~/.requesty directory, where
// a harness may keep files it needs at launch without touching the
// harness's own configuration.
func requestyDirInHome(elements ...string) (string, error) {
	return configDirInHome(append([]string{".requesty"}, elements...)...)
}
