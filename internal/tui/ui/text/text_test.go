package text

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestSplitLine(t *testing.T) {
	testCases := []struct {
		name     string
		left     string
		right    string
		width    int
		expected string
	}{
		{
			name:     "fills available width",
			left:     "Models",
			right:    "10 total",
			width:    20,
			expected: "Models      10 total",
		},
		{
			name:     "keeps one space when width is too narrow",
			left:     "Models",
			right:    "10 total",
			width:    10,
			expected: "Models 10 total",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, RenderSplitLine(tc.left, tc.right, tc.width))
		})
	}
}

func TestSplitLineMeasuresStyledTextByVisibleWidth(t *testing.T) {
	left := lipgloss.NewStyle().Bold(true).Render("Left")
	right := lipgloss.NewStyle().Underline(true).Render("Right")

	line := RenderSplitLine(left, right, 20)

	assert.Equal(t, "Left           Right", ansi.Strip(line))
	assert.Equal(t, 20, lipgloss.Width(line))
}

func TestSection(t *testing.T) {
	section := RenderSplitHeaderSection("Catalogue", "593 models", 24)

	assert.Equal(t, "Catalogue     593 models", ansi.Strip(section))
	assert.Equal(t, 24, lipgloss.Width(section))
}

func TestKeys(t *testing.T) {
	keys := RenderFooterHintList(
		[2]string{"↑/↓", "select"},
		[2]string{"q", "quit"},
	)

	assert.Equal(t, "↑/↓ select · q quit", ansi.Strip(keys))
}
