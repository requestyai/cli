package card

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/assert"
)

func TestRenderUsesRequestedWidth(t *testing.T) {
	rendered := Render("SPEND", "$12.34", 20)

	assert.Equal(t, 20, lipgloss.Width(rendered))
	assert.Contains(t, rendered, "SPEND")
	assert.Contains(t, rendered, "$12.34")

	lines := strings.Split(ansi.Strip(rendered), "\n")
	assert.Greater(t, strings.Index(lines[1], "SPEND"), 5)
	assert.Equal(t, "│                  │", lines[2])
	assert.Greater(t, strings.Index(lines[3], "$12.34"), 5)
}
