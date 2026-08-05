package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/tui/pages/onboarding/apikey"
)

// frame is the breathing room around everything the UI draws.
var frame = lipgloss.NewStyle().Padding(1, 2)

const (
	framePadX     = 2
	framePadY     = 1
	windowWidth   = 80
	windowHeight  = 24
	contentWidth  = windowWidth - 2*framePadX
	contentHeight = windowHeight - 2*framePadY
)

// Root delegates the session to whichever app is currently active.
type Root struct {
	active tea.Model

	// width and height are the drawable area inside the outer frame.
	width, height int
}

// NewRoot builds the UI. A config, saved by an earlier run, skips
// onboarding.
func NewRoot(cfg config.Config) Root {
	root := Root{
		active: NewOnboardingApp(),
		width:  contentWidth,
		height: contentHeight,
	}

	if cfg.APIKey != "" {
		root.active = NewRequestyApp(client.New(cfg))
	}

	return root
}

func (r Root) Init() tea.Cmd {
	return r.active.Init()
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = max(msg.Width-2*framePadX, 0)
		r.height = max(msg.Height-2*framePadY, 0)
		msg = tea.WindowSizeMsg{Width: r.width, Height: r.height}

	case tea.KeyPressMsg:
		switch msg.String() {
		case "ctrl+c":
			return r, tea.Quit
		}

	case apikey.DoneMsg:
		r.active = NewRequestyApp(client.New(msg.Config))
		r.active, _ = r.active.Update(tea.WindowSizeMsg{
			Width:  r.width,
			Height: r.height,
		})

		return r, r.active.Init()
	}

	var cmd tea.Cmd
	r.active, cmd = r.active.Update(msg)

	return r, cmd
}

func (r Root) View() tea.View {
	view := r.active.View()
	view.SetContent(frame.Render(view.Content))
	view.AltScreen = true

	return view
}
