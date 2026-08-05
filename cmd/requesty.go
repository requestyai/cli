package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui"
)

// Run starts the Requesty terminal UI.
func Run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if _, err := tea.NewProgram(tui.NewRoot(cfg)).Run(); err != nil {
		return fmt.Errorf("failed to run program: %w", err)
	}

	return nil
}
