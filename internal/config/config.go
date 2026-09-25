package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	dirName  = ".requesty"
	fileName = "config.json"

	DefaultRouterBaseURL = "https://router.requesty.ai"
	DefaultAPIBaseURL    = "https://api-v2.requesty.ai"
)

// ErrNoProfiles means nobody has signed in on this machine yet.
var ErrNoProfiles = errors.New("no Requesty profile configured; run `requesty login`")

// Config is what a client or harness runs with: an API key and the router it
// is sent to. The settings file holds one per profile.
type Config struct {
	Name          string `json:"-"`
	APIKey        string `json:"api_key"`
	RouterBaseURL string `json:"router_base_url"`
	// HarnessModels is the model each harness was picked to launch with.
	HarnessModels map[string]string `json:"harness_models,omitempty"`
	// HarnessFastModels is the model each harness hands background work to,
	// for harnesses that have such a slot.
	HarnessFastModels map[string]string `json:"harness_fast_models,omitempty"`
}

// SetHarnessModel remembers model as the one to launch harness with.
func (c *Config) SetHarnessModel(harness, model string) {
	if c.HarnessModels == nil {
		c.HarnessModels = make(map[string]string)
	}
	c.HarnessModels[harness] = model
}

// SetHarnessFastModel remembers model as the one harness hands background
// work to.
func (c *Config) SetHarnessFastModel(harness, model string) {
	if c.HarnessFastModels == nil {
		c.HarnessFastModels = make(map[string]string)
	}
	c.HarnessFastModels[harness] = model
}

// APIBaseURL is the management API that corresponds to the router: the host's
// "router" becomes "api-v2" and the local stack's port 40000 becomes 40003.
// The path is left alone, and an address that cannot be parsed comes back as
// it was given.
func (c Config) APIBaseURL() string {
	parsed, err := url.Parse(c.RouterBaseURL)
	if err != nil || parsed.Host == "" {
		return c.RouterBaseURL
	}

	host, port := parsed.Hostname(), parsed.Port()
	rewritten := strings.Replace(host, "router", "api-v2", 1)
	if port == "40000" {
		port = "40003"
	}
	if rewritten == host && port == parsed.Port() {
		return c.RouterBaseURL
	}

	if port != "" {
		parsed.Host = net.JoinHostPort(rewritten, port)
	} else {
		parsed.Host = rewritten
	}

	return parsed.String()
}

// Store is the saved profiles and which one is current.
type Store struct {
	Current  string            `json:"current"`
	Profiles map[string]Config `json:"profiles"`
}

// Names lists the saved profiles in a stable order.
func (s Store) Names() []string {
	names := make([]string, 0, len(s.Profiles))
	for name := range s.Profiles {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

// Resolve returns the named profile, or the current profile when name is
// empty.
func (s Store) Resolve(name string) (Config, error) {
	if len(s.Profiles) == 0 {
		return Config{}, ErrNoProfiles
	}
	if name == "" {
		name = s.Current
	}

	cfg, ok := s.Profiles[name]
	if !ok {
		return Config{}, fmt.Errorf("profile %q not found; saved profiles: %s", name, strings.Join(s.Names(), ", "))
	}
	cfg.Name = name

	return cfg, nil
}

// Set stores cfg as the named profile, replacing any existing one.
func (s *Store) Set(name string, cfg Config) {
	if s.Profiles == nil {
		s.Profiles = make(map[string]Config)
	}
	cfg.Name = ""
	s.Profiles[name] = cfg
	if s.Current == "" {
		s.Current = name
	}
}

// Use makes name the current profile.
func (s *Store) Use(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found; saved profiles: %s", name, strings.Join(s.Names(), ", "))
	}
	s.Current = name
	return nil
}

// Remove deletes name and keeps Current valid. If the current profile is
// removed, the first remaining profile in name order becomes current.
func (s *Store) Remove(name string) error {
	if _, ok := s.Profiles[name]; !ok {
		return fmt.Errorf("profile %q not found; saved profiles: %s", name, strings.Join(s.Names(), ", "))
	}
	delete(s.Profiles, name)
	if s.Current == name {
		s.Current = ""
		if names := s.Names(); len(names) > 0 {
			s.Current = names[0]
		}
	}
	return nil
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

// Load reads the settings and fills in the production router for profiles
// that leave it blank. A missing file is not an error: it means the user has
// not onboarded yet, so an empty Store comes back.
func Load() (Store, error) {
	path, err := configPath()
	if err != nil {
		return Store{}, err
	}

	var store Store
	data, err := os.ReadFile(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		// Not onboarded yet.
	case err != nil:
		return Store{}, fmt.Errorf("failed to read config: %w", err)
	default:
		if err := json.Unmarshal(data, &store); err != nil {
			return Store{}, fmt.Errorf("failed to parse config: %w", err)
		}
		if err := validate(store); err != nil {
			return Store{}, err
		}
	}

	for name, cfg := range store.Profiles {
		if cfg.RouterBaseURL == "" {
			cfg.RouterBaseURL = DefaultRouterBaseURL
			store.Profiles[name] = cfg
		}
	}

	return store, nil
}

// Save writes the settings, creating ~/.requesty if it is missing.
func Save(store Store) error {
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// validate reports a store that names no current profile, or names one that
// is not saved. An empty store is the not-onboarded state and is fine.
func validate(store Store) error {
	if len(store.Profiles) > 0 && store.Current == "" {
		return errors.New("current profile is required")
	}
	if store.Current != "" {
		if _, ok := store.Profiles[store.Current]; !ok {
			return fmt.Errorf("current profile %q is not saved", store.Current)
		}
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
