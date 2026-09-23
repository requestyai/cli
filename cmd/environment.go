package cmd

import (
	"errors"
	"fmt"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/modelpicker"
	"github.com/requestyai/cli/internal/onboarding"
	"github.com/spf13/cobra"
)

const (
	// profileFlag names the saved profile to run with. It lives on the root
	// so every subcommand inherits it; harness commands parse it themselves.
	profileFlag = "profile"
	// profileEnv is the environment variable that stands in for --profile.
	profileEnv = "REQUESTY_PROFILE"
	// defaultProfileName is what onboarding calls the first profile.
	defaultProfileName = "default"
)

// environment is the profile store and, once a profile has been picked, the
// session the commands share.
type environment struct {
	store   config.Store
	session *session
}

type session struct {
	config config.Config
	client *client.Client
}

func newSession(cfg config.Config) *session {
	return &session{config: cfg, client: client.New(cfg)}
}

func newEnvironment() (*environment, error) {
	store, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return &environment{store: store}, nil
}

// requireProfile is the PersistentPreRunE of the command groups that cannot
// run without a key. It resolves the profile once and hands the commands a
// client for it.
func (env *environment) requireProfile(cmd *cobra.Command, _ []string) error {
	cfg, err := env.resolveProfile(cmd, "")
	if err != nil {
		return err
	}
	env.session = newSession(cfg)

	return nil
}

// resolveProfile returns the profile selected for this command.
func (env *environment) resolveProfile(cmd *cobra.Command, name string) (config.Config, error) {
	if name == "" {
		name = requestedProfile(cmd)
	}

	return env.store.Resolve(name)
}

// ensureProfile returns the selected profile, running onboarding when none
// has been saved yet.
func (env *environment) ensureProfile(cmd *cobra.Command, name, harness string) (config.Config, error) {
	cfg, err := env.resolveProfile(cmd, name)
	if !errors.Is(err, config.ErrNoProfiles) {
		return cfg, err
	}

	cfg, err = onboarding.Run(cmd.Context(), onboarding.Options{
		Config:  config.Config{RouterBaseURL: config.DefaultRouterBaseURL},
		Harness: harness,
	})
	if err != nil {
		return config.Config{}, err
	}
	if err := env.save(defaultProfileName, cfg); err != nil {
		return config.Config{}, err
	}
	cfg.Name = defaultProfileName

	return cfg, nil
}

// ensureModel returns the model the harness launches with, running the
// picker when the profile has none for it yet and saving the answer.
func (env *environment) ensureModel(cmd *cobra.Command, cfg config.Config, spec harnessSpec, parsed harnessArgs) (string, error) {
	model, ask := modelToLaunch(cfg, spec, parsed)
	if !ask {
		return model, nil
	}

	picked, err := modelpicker.Run(cmd.Context(), modelpicker.Options{
		Client:    client.New(cfg),
		Harness:   spec.displayName,
		Preselect: model,
	})
	if errors.Is(err, modelpicker.ErrCancelled) {
		return "", err
	}
	if err != nil {
		// Most often there is no terminal to ask in, such as a script or CI.
		return "", fmt.Errorf("no model picked for %s in profile %q; pass %s <id> or run in a terminal once (%w)", spec.displayName, cfg.Name, harnessModelFlag, err)
	}

	cfg.SetHarnessModel(spec.binary, picked)
	if err := env.save(cfg.Name, cfg); err != nil {
		return "", err
	}

	return picked, nil
}

// modelToLaunch applies the rules for which model a harness launches with:
// --model for this run only, else the model picked for this harness in the
// profile, else ask. When asking, model is what the picker should suggest:
// the saved model that --choose-model is replacing, or the harness default.
func modelToLaunch(cfg config.Config, spec harnessSpec, parsed harnessArgs) (model string, ask bool) {
	if parsed.launch.Model != "" {
		return parsed.launch.Model, false
	}

	saved := cfg.HarnessModels[spec.binary]
	if saved != "" && !parsed.chooseModel {
		return saved, false
	}
	if saved != "" {
		return saved, true
	}

	return spec.defaultModel, true
}

// save stores cfg as the named profile and writes the file. The first profile
// saved becomes current.
func (env *environment) save(name string, cfg config.Config) error {
	env.store.Set(name, cfg)
	if err := config.Save(env.store); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	return nil
}
