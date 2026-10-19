package harnesses

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttributionHeadersNameHarnessAndClient(t *testing.T) {
	assert.Equal(t, map[string]string{
		"X-Title":                   "Tool",
		"X-Requesty-Client":         "requesty-cli",
		"X-Requesty-Client-Version": "dev",
	}, attributionHeaders("Tool"))
}

func TestMergePatchMergesTypedHeadersIntoExisting(t *testing.T) {
	destination := map[string]any{
		"headers": map[string]any{"X-Team": "platform", "X-Title": "stale"},
	}

	require.NoError(t, mergePatch(destination, map[string]any{
		"headers": attributionHeaders("Tool"),
	}, ""))

	assert.Equal(t, map[string]any{
		"headers": map[string]any{
			"X-Team":                    "platform",
			"X-Title":                   "Tool",
			"X-Requesty-Client":         "requesty-cli",
			"X-Requesty-Client-Version": "dev",
		},
	}, destination)
}
