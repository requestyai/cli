package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

const (
	dirName  = ".requesty"
	fileName = "config.json"

	DefaultRouterBaseURL = "https://router.requesty.ai"
	DefaultAPIBaseURL    = "https://api-v2.requesty.ai"
)

// Config is the settings file.
type Config struct {
	APIKey        string `json:"api_key"`
	RouterBaseURL string `json:"router_base_url"`
	APIBaseURL    string `json:"api_base_url,omitempty"`
}

// Load reads the settings. A missing file is not an error: it means the user
// has not onboarded yet, so the zero Config comes back.
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return Config{}, fmt.Errorf("failed to parse config: %w", err)
	}

	return config, nil
}

// Save writes the settings, creating ~/.requesty if it is missing.
func Save(config Config) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

func configPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to find home directory: %w", err)
	}

	return filepath.Join(home, dirName, fileName), nil
}
