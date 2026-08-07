package card

import (
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/stretchr/testify/assert"
)

func TestRenderUsesRequestedWidth(t *testing.T) {
	rendered := Render("SPEND", "$12.34", 20)

	assert.Equal(t, 20, lipgloss.Width(rendered))
	assert.Contains(t, rendered, "SPEND")
	assert.Contains(t, rendered, "$12.34")
}
