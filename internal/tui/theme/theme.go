package theme

import "charm.land/lipgloss/v2"

// Palette.
var (
	Bg       = lipgloss.Color("#0B0F17")
	BgPanel  = lipgloss.Color("#121A28")
	BgSelect = lipgloss.Color("#18253D")
	Line     = lipgloss.Color("#1E2A3D")

	Blue     = lipgloss.Color("#3B82F6")
	BlueDeep = lipgloss.Color("#1D4ED8")
	BlueLite = lipgloss.Color("#7DA9FF")

	Text  = lipgloss.Color("#E6EAF3")
	Dim   = lipgloss.Color("#8A95AC")
	Faint = lipgloss.Color("#59637A")

	Green = lipgloss.Color("#34D399")
	Red   = lipgloss.Color("#F87171")
	Amber = lipgloss.Color("#FBBF24")
)

// Styles.
var (
	Body    = lipgloss.NewStyle().Foreground(Text)
	Heading = lipgloss.NewStyle().Foreground(Text).Bold(true)
	Label   = lipgloss.NewStyle().Foreground(Dim)
	Muted   = lipgloss.NewStyle().Foreground(Faint)
	Accent  = lipgloss.NewStyle().Foreground(Blue)
	Bright  = lipgloss.NewStyle().Foreground(BlueLite)

	Good = lipgloss.NewStyle().Foreground(Green)
	Bad  = lipgloss.NewStyle().Foreground(Red)
	Warn = lipgloss.NewStyle().Foreground(Amber)

	// Panel is the bordered container used for cards and detail boxes.
	Panel = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(Line).
		Background(BgPanel).
		Padding(0, 1)

	// Rule is a full-width horizontal divider.
	Rule = lipgloss.NewStyle().Foreground(Line)

	// Pill marks a selected choice in an inline switcher, such as the time
	// window. PillOff is its unselected counterpart.
	Pill = lipgloss.NewStyle().
		Foreground(BlueLite).
		Background(BgSelect).
		Padding(0, 1)

	PillOff = lipgloss.NewStyle().
		Foreground(Faint).
		Padding(0, 1)

	Footer = lipgloss.NewStyle().Foreground(Faint)
	Key    = lipgloss.NewStyle().Foreground(Dim).Bold(true)
)
