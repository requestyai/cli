package onboarding

import (
	"context"
	"errors"
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/config"
)

var (
	// ErrCancelled reports that interactive onboarding ended before a key was
	// saved.
	ErrCancelled = errors.New("sign-in cancelled")
)

// Run signs a user in with their browser and takes them through to a new key
// in the full-screen UI. The caller saves the config it returns.
func Run(ctx context.Context, opts Options) (config.Config, error) {
	final, err := tea.NewProgram(newModel(ctx, opts), tea.WithContext(ctx), tea.WithOutput(os.Stderr)).Run()
	if err != nil {
		return config.Config{}, fmt.Errorf("failed to run onboarding: %w", err)
	}

	finished, ok := final.(model)
	if !ok || finished.done == nil {
		return config.Config{}, ErrCancelled
	}
	return *finished.done, nil
}

// RunHeadless signs a user in with their browser using plain-text prompts and
// status output. The caller saves the config it returns.
func RunHeadless(ctx context.Context, opts Options) (config.Config, error) {
	_, _ = fmt.Fprintf(os.Stderr,
		"Signing in with your browser will create an API key named %q in your Requesty\naccount and save it to %s.\n\n",
		defaultKeyName(), config.DisplayPath())

	r, err := provision(ctx, opts)
	if err != nil {
		return config.Config{}, err
	}

	if r.Group != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Created API key %q in group %q.\nManage it at %s.\n", r.KeyName, r.Group.Name, APIKeysURL)
	} else {
		_, _ = fmt.Fprintf(os.Stderr, "Created API key %q in your personal API keys.\nManage it at %s.\n", r.KeyName, APIKeysURL)
	}

	return r.Config, nil
}
