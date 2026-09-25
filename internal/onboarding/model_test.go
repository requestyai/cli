package onboarding

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func key(name string) tea.KeyPressMsg {
	switch name {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		return tea.KeyPressMsg{Code: rune(name[0]), Text: name}
	}
}

func update(m model, msg tea.Msg) (model, tea.Cmd) {
	next, cmd := m.Update(msg)
	return next.(model), cmd
}

func signingIn() model {
	m := newModel(context.Background(), Options{})
	m.dialog = dialogState{open: true, step: dialogSigningIn, cancel: func() {}}
	return m
}

func view(m model) string {
	return ansi.Strip(m.View().Content)
}

func TestSignInStartsImmediately(t *testing.T) {
	m := newModel(context.Background(), Options{Harness: "Claude Code"})

	assert.True(t, m.dialog.open)
	assert.Equal(t, dialogSigningIn, m.dialog.step)
	assert.NotNil(t, m.dialog.cancel)
	assert.NotNil(t, m.Init(), "Init hands the runtime the sign-in")
	out := view(m)
	assert.Contains(t, out, "Welcome to Requesty")
	assert.Contains(t, out, "One gateway for every model")
	assert.Contains(t, out, "Waiting for you to finish signing in")
	assert.Contains(t, out, "Claude Code starts as soon as the key is saved")
	assert.Contains(t, out, "--api-key <key>")
}

func TestSigningInDialogShowsAuthorizeURL(t *testing.T) {
	m := signingIn()

	m, _ = update(m, authorizeURLMsg{url: "https://api.example/authorize"})

	assert.Contains(t, view(m), "If it did not open")
	assert.Contains(t, view(m), "https://api.example/authorize")
}

func TestAuthorizeURLIsOneHyperlinkWhenWrapped(t *testing.T) {
	m := signingIn()
	long := "https://api.example/authorize?" + strings.Repeat("x=1&", 40)

	m, _ = update(m, authorizeURLMsg{url: long})
	raw := m.View().Content

	assert.Contains(t, ansi.Strip(raw), long[:20])
	open := ansi.SetHyperlink(long, authorizeLinkID)
	opens := strings.Count(raw, open)
	assert.Greater(t, opens, 1, "each wrapped line is its own link to the full URL, sharing an id")
	assert.Equal(t, opens, strings.Count(raw, ansi.ResetHyperlink()), "every line closes its link")
	for _, segment := range strings.Split(raw, open)[1:] {
		linked, _, found := strings.Cut(segment, ansi.ResetHyperlink())
		require.True(t, found)
		assert.NotContains(t, linked, "\n", "a link never runs past its line into padding or borders")
	}
}

func TestEscapeCancelsAndQuits(t *testing.T) {
	m := signingIn()
	cancelled := false
	m.dialog.cancel = func() { cancelled = true }

	m, cmd := update(m, key("esc"))

	assert.True(t, cancelled)
	assert.NotNil(t, cmd, "quits")
	assert.Nil(t, m.done)
	assert.NoError(t, m.err, "a cancel is not an error")
}

func TestSignInFailureQuitsWithError(t *testing.T) {
	m := signingIn()

	m, cmd := update(m, signedInMsg{err: errors.New("access_denied")})

	assert.NotNil(t, cmd, "quits")
	assert.EqualError(t, m.err, "access_denied")
	assert.Nil(t, m.done)
}

func TestSeveralGroupsOpenPickerAndMoveCursor(t *testing.T) {
	s := &session{groups: []client.Group{
		group("g-1", "Engineering"),
		group("g-2", "Research"),
	}}
	m := signingIn()

	m, cmd := update(m, signedInMsg{session: s})
	require.Nil(t, cmd)
	assert.Equal(t, dialogChooseGroup, m.dialog.step)
	assert.Contains(t, view(m), "Engineering")
	assert.Contains(t, view(m), "Research")

	m, _ = update(m, key("down"))
	m, _ = update(m, key("down"))
	assert.Equal(t, 1, m.dialog.groupCursor)
	m, _ = update(m, key("up"))
	assert.Equal(t, 0, m.dialog.groupCursor)

	m, cmd = update(m, key("enter"))
	assert.Equal(t, dialogSaving, m.dialog.step)
	assert.NotNil(t, cmd)
}

func TestSavedResultQuitsWithConfig(t *testing.T) {
	m := newModel(context.Background(), Options{})
	m.dialog = dialogState{open: true, step: dialogSaving}
	cfg := config.Config{APIKey: "rqsty-new"}

	m, cmd := update(m, savedMsg{result: result{Config: cfg}})

	require.NotNil(t, cmd)
	assert.Equal(t, cfg, *m.done)
}

func TestKeysAreIgnoredWhileSaving(t *testing.T) {
	m := newModel(context.Background(), Options{})
	m.dialog = dialogState{open: true, step: dialogSaving}

	m, cmd := update(m, key("esc"))

	assert.Nil(t, cmd)
	assert.True(t, m.dialog.open)
}
