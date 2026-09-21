package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
}

func (c Config) APIBaseURL() string {
	apiBaseURL := strings.Replace(c.RouterBaseURL, "router", "api-v2", 1)
	apiBaseURL = strings.Replace(apiBaseURL, "40000", "40003", 1)

	return apiBaseURL
}

// DisplayPath is where the settings file lives, for messages: the real
// location when it can be worked out, else the conventional one.
func DisplayPath() string {
	path, err := configPath()
	if err != nil {
		return "~/" + dirName + "/" + fileName
	}

	return path
}

// Load reads the settings and fills in the production router when the file
// leaves it blank. A missing file is not an error: it means the user has not
// onboarded yet, so the defaults come back with no API key.
func Load() (Config, error) {
	path, err := configPath()
	if err != nil {
		return Config{}, err
	}

	var config Config
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Not onboarded yet: defaults only.
	case err != nil:
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	default:
		if err := json.Unmarshal(data, &config); err != nil {
			return Config{}, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	if config.RouterBaseURL == "" {
		config.RouterBaseURL = DefaultRouterBaseURL
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
