package onboarding

import (
	"context"
	"errors"
	"fmt"
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

func TestWelcomeExplainsOnboarding(t *testing.T) {
	out := view(newModel(context.Background(), Options{Harness: "Claude Code"}))

	assert.Contains(t, out, "Welcome to Requesty")
	assert.Contains(t, out, fmt.Sprintf("%q", defaultKeyName()))
	assert.Contains(t, out, "Claude Code starts as soon as the key is saved")
	assert.Contains(t, out, "--api-key <key>")
}

func TestEnterOpensSigningInDialog(t *testing.T) {
	m, cmd := update(newModel(context.Background(), Options{}), key("enter"))

	assert.True(t, m.dialog.open)
	assert.Equal(t, dialogSigningIn, m.dialog.step)
	assert.NotNil(t, m.dialog.cancel)
	assert.NotNil(t, cmd)
	assert.Contains(t, view(m), "Waiting for you to finish signing in")
}

func TestSigningInDialogShowsAuthorizeURL(t *testing.T) {
	m := signingIn()

	m, _ = update(m, authorizeURLMsg{url: "https://api.example/authorize"})

	assert.Contains(t, view(m), "If it did not open")
	assert.Contains(t, view(m), "https://api.example/authorize")
}

func TestEscapeCancelsAndIgnoresLateSignIn(t *testing.T) {
	m := signingIn()
	cancelled := false
	m.dialog.cancel = func() { cancelled = true }

	m, _ = update(m, key("esc"))
	assert.True(t, cancelled)
	assert.False(t, m.dialog.open)

	m, cmd := update(m, signedInMsg{session: &session{}})
	assert.False(t, m.dialog.open)
	assert.Nil(t, cmd)
}

func TestSignInFailureReturnsToWelcome(t *testing.T) {
	m := signingIn()

	m, _ = update(m, signedInMsg{err: errors.New("access_denied")})

	assert.False(t, m.dialog.open)
	assert.Contains(t, view(m), "access_denied")
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
