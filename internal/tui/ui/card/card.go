package card

import (
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/tui/theme"
)

// Render draws a compact, fixed-width summary card.
func Render(title, value string, width int) string {
	innerWidth := max(width-theme.Panel.GetHorizontalFrameSize(), 1)
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		theme.Label.Bold(true).Width(innerWidth).Align(lipgloss.Center).Render(title),
		"",
		theme.Heading.Width(innerWidth).Align(lipgloss.Center).Render(value),
	)

	return theme.Panel.Width(max(width, 1)).Render(body)
}
