package text

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/requestyai/cli/internal/tui/theme"
	"github.com/requestyai/cli/internal/version"
)

const LineSeparator = ""
const SpaceSeparator = " "

// RenderSplitHeaderSection is a heading above a block of content, with
// the right-hand note set against the far edge of the given width.
func RenderSplitHeaderSection(title, note string, width int) string {
	return RenderSplitLine(theme.Heading.Render(title), theme.Muted.Render(note), width)
}

// RenderSplitLine pushes two already-styled fragments to opposite ends of a width.
func RenderSplitLine(left, right string, width int) string {
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 1 {
		gap = 1
	}

	return left + strings.Repeat(" ", gap) + right
}

// RenderFooterHintList renders a footer hint list: "↑/↓ select · q quit"
// with a version number on the right.
func RenderFooterHintList(width int, pairs ...[2]string) string {
	out := make([]string, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, theme.Key.Render(p[0])+" "+theme.Footer.Render(p[1]))
	}

	return RenderSplitLine(
		strings.Join(out, theme.Footer.Render(" · ")),
		theme.Footer.Render(version.Version),
		width,
	)
}
