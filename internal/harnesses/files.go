package harnesses

import (
	"encoding/json"
	"fmt"

	"github.com/pelletier/go-toml/v2"
	"github.com/requestyai/cli/internal/fileio"
	"gopkg.in/yaml.v3"
)

// configFilePerm keeps harness config files readable by their owner only, since
// most of them hold the Requesty API key.
const configFilePerm = 0o600

func backupAndWriteConfigFileAsJSON(path string, data any) error {
	dataBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	return backupAndWriteFile(path, dataBytes)
}

func backupAndWriteConfigFileAsTOML(path string, data any) error {
	dataBytes, err := toml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	return backupAndWriteFile(path, dataBytes)
}

func backupAndWriteConfigFileAsYAML(path string, data any) error {
	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	return backupAndWriteFile(path, dataBytes)
}

func backupAndWriteFile(path string, data []byte) error {
	return fileio.BackupAndWrite(path, data, configFilePerm)
}

func pathExists(path string) (bool, error) {
	return fileio.Exists(path)
}
