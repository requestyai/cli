package card

import (
	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/tui/theme"
)

// Render draws a compact, fixed-width summary card.
func Render(title, value string, width int) string {
	body := lipgloss.JoinVertical(
		lipgloss.Left,
		theme.Label.Bold(true).Render(title),
		theme.Heading.Render(value),
	)

	return theme.Panel.Width(max(width, 1)).Render(body)
}
