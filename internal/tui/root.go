package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/config"
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

// Root frames the configured Requesty app.
type Root struct {
	active tea.Model

	// width and height are the drawable area inside the outer frame.
	width, height int
}

// NewRoot builds the configured Requesty UI.
func NewRoot(cfg config.Config) Root {
	return Root{
		active: NewRequestyApp(cfg),
		width:  contentWidth,
		height: contentHeight,
	}
}

func (r Root) Init() tea.Cmd {
	return r.active.Init()
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch typedMsg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width = max(typedMsg.Width-2*framePadX, 0)
		r.height = max(typedMsg.Height-2*framePadY, 0)

		// Apps draw inside the frame, so they are sized to the content
		// area rather than the terminal.
		msg = tea.WindowSizeMsg{Width: r.width, Height: r.height}

	case tea.KeyPressMsg:
		switch typedMsg.String() {
		case "ctrl+c":
			return r, tea.Quit
		}
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
