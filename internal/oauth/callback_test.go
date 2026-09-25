package oauth

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// visit plays the browser: it follows the redirect the authorization server
// would have issued and returns what the callback page said.
func visit(t *testing.T, server *callbackServer, query url.Values) (int, string) {
	t.Helper()

	resp, err := http.Get(server.RedirectURI() + "?" + query.Encode())
	require.NoError(t, err)
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp.StatusCode, string(body)
}

func TestCallbackRedirectURIUsesLoopbackIPLiteral(t *testing.T) {
	server, err := listenCallback("state")
	require.NoError(t, err)
	defer server.Close()

	redirect, err := url.Parse(server.RedirectURI())
	require.NoError(t, err)

	assert.Equal(t, "http", redirect.Scheme)
	assert.Equal(t, "127.0.0.1", redirect.Hostname())
	assert.NotEmpty(t, redirect.Port())
	assert.Equal(t, "/callback", redirect.Path)
}

func TestCallbackDeliversCode(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	status, body := visit(t, server, url.Values{
		"code":  {"the-code"},
		"state": {"expected-state"},
		"iss":   {"https://api-v2.requesty.ai"},
	})
	assert.Equal(t, http.StatusOK, status)
	assert.Contains(t, body, "You&#39;re signed in to Requesty CLI. You can close this tab.")

	code, err := server.Wait(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "the-code", code)
}

func TestCallbackRejectsStateMismatch(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	status, body := visit(t, server, url.Values{
		"code":  {"the-code"},
		"state": {"forged-state"},
	})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, body, "state mismatch")

	code, err := server.Wait(context.Background())
	require.ErrorContains(t, err, "state mismatch")
	assert.Empty(t, code)
}

func TestCallbackSurfacesErrorParameters(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	status, body := visit(t, server, url.Values{
		"error":             {"access_denied"},
		"error_description": {"the user denied the request"},
		"state":             {"expected-state"},
	})
	assert.Equal(t, http.StatusBadRequest, status)
	assert.Contains(t, body, "access_denied: the user denied the request")

	_, err = server.Wait(context.Background())
	var oauthErr *Error
	require.ErrorAs(t, err, &oauthErr)
	assert.Equal(t, "access_denied", oauthErr.Code)
	assert.Equal(t, "the user denied the request", oauthErr.Description)
}

func TestCallbackRejectsResponseWithoutCode(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	visit(t, server, url.Values{"state": {"expected-state"}})

	_, err = server.Wait(context.Background())
	require.ErrorContains(t, err, "no code")
}

func TestCallbackEscapesHTMLInErrors(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	_, body := visit(t, server, url.Values{
		"error":             {"access_denied"},
		"error_description": {"<script>alert(1)</script>"},
		"state":             {"expected-state"},
	})

	assert.False(t, strings.Contains(body, "<script>"), "error description must be escaped: %s", body)
	assert.Contains(t, body, "&lt;script&gt;")
}

func TestCallbackKeepsFirstOutcome(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	visit(t, server, url.Values{"code": {"first"}, "state": {"expected-state"}})
	visit(t, server, url.Values{"code": {"second"}, "state": {"expected-state"}})

	code, err := server.Wait(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "first", code)
}

func TestCallbackIgnoresOtherPaths(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	resp, err := http.Get(strings.TrimSuffix(server.RedirectURI(), callbackPath) + "/favicon.ico")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = server.Wait(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

func TestCallbackWaitStopsWhenContextIsCancelled(t *testing.T) {
	server, err := listenCallback("expected-state")
	require.NoError(t, err)
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = server.Wait(ctx)
	require.ErrorIs(t, err, context.Canceled)
}
