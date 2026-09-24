package cmd

import (
	"errors"
	"fmt"
	"slices"

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
	// defaultProfileName is what onboarding calls the first profile.
	defaultProfileName = "default"
)

// environment is the profile store and, once a profile has been picked, the
// session the commands share.
type environment struct {
	store   config.Store
	session *session
}

// session is the resolved profile and the client built from it, set once the
// profile for a command has been picked.
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
		name, _ = cmd.Flags().GetString(profileFlag)
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

// ensureModels returns the model the harness launches with and, for
// harnesses that have one, the model its background work goes to.
func (env *environment) ensureModels(cmd *cobra.Command, cfg config.Config, spec harnessSpec, parsed harnessArgs) (model, fast string, err error) {
	model, pickedModel, err := env.ensureModel(cmd, cfg, modelRequest{
		displayName:  spec.displayName,
		override:     parsed.launch.Model,
		overrideFlag: harnessModelFlag,
		ask:          parsed.chooseModel,
		askFlag:      harnessChooseModelFlag,
		saved:        cfg.HarnessModels[spec.binary],
		defaults:     spec.defaultModels,
	})
	if err != nil {
		return "", "", err
	}
	if pickedModel {
		cfg.SetHarnessModel(spec.binary, model)
		// A new main model may sit in another region, so the fast model is
		// forgotten and picked again below to go with it.
		delete(cfg.HarnessFastModels, spec.binary)
	}

	pickedFast := false
	if spec.hasFastModel() {
		// The main model is the last resort: it is known to be permitted,
		// so an access list without any smaller model still works.
		fast, pickedFast, err = env.ensureModel(cmd, cfg, modelRequest{
			displayName:  spec.displayName + " background work",
			override:     parsed.launch.FastModel,
			overrideFlag: harnessFastModelFlag,
			ask:          parsed.chooseFastModel,
			askFlag:      harnessChooseFastModelFlag,
			saved:        cfg.HarnessFastModels[spec.binary],
			defaults:     slices.Concat(spec.defaultFastModels, []string{model}),
		})
		if err != nil {
			return "", "", err
		}
		if pickedFast {
			cfg.SetHarnessFastModel(spec.binary, fast)
		}
	}

	if pickedModel || pickedFast {
		if err := env.save(cfg.Name, cfg); err != nil {
			return "", "", err
		}
	}

	return model, fast, nil
}

// modelRequest is how one of a harness's models was asked for on this run.
type modelRequest struct {
	// displayName is shown in the picker title and in messages: "Claude Code",
	// "Claude Code background work".
	displayName string
	// override is the model for this run only, from overrideFlag.
	override string
	// overrideFlag is "--model" or "--fast-model", for messages.
	overrideFlag string
	// ask says askFlag was passed: open the picker even if saved.
	ask bool
	// askFlag is "--choose-model" or "--choose-fast-model", for messages.
	askFlag string
	// saved is what the profile remembers, if anything.
	saved string
	// defaults are tried in order when nothing is saved; the first one the
	// profile can route to is used without asking.
	defaults []string
}

// ensureModel returns the model for one request: the override for this run
// only, else the saved one, else whatever the picker decides, which it
// reports through picked so the caller knows to save it.
func (env *environment) ensureModel(cmd *cobra.Command, cfg config.Config, req modelRequest) (model string, picked bool, err error) {
	switch {
	case req.override != "":
		return req.override, false, nil
	case req.saved != "" && !req.ask:
		return req.saved, false, nil
	}

	preferred := req.defaults
	if req.saved != "" {
		preferred = slices.Concat([]string{req.saved}, req.defaults)
	}
	model, asked, err := modelpicker.Run(cmd.Context(), modelpicker.Options{
		Client:    client.New(cfg),
		Harness:   req.displayName,
		Preferred: preferred,
		Confirm:   req.ask,
	})
	if errors.Is(err, modelpicker.ErrCancelled) {
		return "", false, err
	}
	if err != nil {
		// Most often there is no terminal to ask in, such as a script or CI.
		return "", false, fmt.Errorf("no model picked for %s in profile %q; pass %s <id> or run in a terminal once (%w)", req.displayName, cfg.Name, req.overrideFlag, err)
	}
	if !asked {
		if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Using %s for %s; change with %s.\n", model, req.displayName, req.askFlag); err != nil {
			return "", false, err
		}
	}

	return model, true, nil
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
