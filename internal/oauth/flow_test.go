package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeBrowser stands in for the user's browser: it fetches the authorize URL
// and follows the redirect back to the CLI's loopback server.
func fakeBrowser(t *testing.T) func(string) error {
	return func(address string) error {
		go func() {
			resp, err := http.Get(address)
			if err != nil {
				t.Errorf("browser failed: %v", err)
				return
			}
			resp.Body.Close()
		}()

		return nil
	}
}

// authServer is a minimal authorization server. It records what the CLI sent
// so tests can assert the wire contract.
type authServer struct {
	t         *testing.T
	authorize url.Values
	tokenForm url.Values
	tokenReq  *http.Request

	// consent decides how the browser is redirected back. nil approves.
	consent func(redirect *url.URL, authorize url.Values)
	// token writes the token response. nil issues a normal token.
	token func(w http.ResponseWriter)
}

func (s *authServer) handler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /v1/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		s.authorize = r.URL.Query()

		redirect, err := url.Parse(s.authorize.Get("redirect_uri"))
		require.NoError(s.t, err)

		if s.consent != nil {
			s.consent(redirect, s.authorize)
		} else {
			query := redirect.Query()
			query.Set("code", "issued-code")
			query.Set("state", s.authorize.Get("state"))
			query.Set("iss", "http://"+r.Host)
			redirect.RawQuery = query.Encode()
		}

		http.Redirect(w, r, redirect.String(), http.StatusFound)
	})

	mux.HandleFunc("POST /v1/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		s.tokenReq = r
		require.NoError(s.t, r.ParseForm())
		s.tokenForm = r.PostForm

		w.Header().Set("Content-Type", "application/json")
		if s.token != nil {
			s.token(w)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  "access-token",
			"token_type":    "Bearer",
			"expires_in":    300,
			"refresh_token": "refresh-token",
			"scope":         Scope,
		})
	})

	return mux
}

func TestLoginCompletesAuthorizationCodeFlow(t *testing.T) {
	auth := &authServer{t: t}
	server := httptest.NewServer(auth.handler())
	defer server.Close()

	var status bytes.Buffer
	token, err := Login(context.Background(), Options{
		APIBaseURL:  server.URL + "/",
		Status:      &status,
		OpenBrowser: fakeBrowser(t),
	})
	require.NoError(t, err)

	assert.Equal(t, "access-token", token.AccessToken)
	assert.Equal(t, "Bearer", token.TokenType)
	assert.Equal(t, 5*time.Minute, token.ExpiresIn)
	assert.Equal(t, []string{"manage:group:r", "manage:apikey:w"}, token.Scopes())

	// The authorize request carries the full OAuth 2.1 + PKCE + RFC 8707 set.
	assert.Equal(t, "code", auth.authorize.Get("response_type"))
	assert.Equal(t, "requesty-cli", auth.authorize.Get("client_id"))
	assert.Equal(t, "manage:group:r manage:apikey:w", auth.authorize.Get("scope"))
	assert.Equal(t, server.URL+"/v1/manage", auth.authorize.Get("resource"))
	assert.Equal(t, "S256", auth.authorize.Get("code_challenge_method"))
	assert.NotEmpty(t, auth.authorize.Get("state"))
	assert.NotEmpty(t, auth.authorize.Get("code_challenge"))

	redirectURI := auth.authorize.Get("redirect_uri")
	assert.True(t, strings.HasPrefix(redirectURI, "http://127.0.0.1:"), redirectURI)
	assert.True(t, strings.HasSuffix(redirectURI, "/callback"), redirectURI)

	// The token request is form-encoded with exactly the fields the backend
	// binds, and the redirect URI matches the authorize request verbatim.
	assert.Equal(t, "application/x-www-form-urlencoded", auth.tokenReq.Header.Get("Content-Type"))
	assert.Empty(t, auth.tokenReq.Header.Get("Authorization"), "public client must not send credentials")
	assert.Equal(t, url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {"issued-code"},
		"redirect_uri":  {redirectURI},
		"client_id":     {"requesty-cli"},
		"code_verifier": {auth.tokenForm.Get("code_verifier")},
	}, auth.tokenForm)

	// The verifier the CLI redeemed is the one behind the challenge it sent.
	assert.Equal(t, auth.authorize.Get("code_challenge"), Challenge(auth.tokenForm.Get("code_verifier")))

	// The URL is always printed so headless users can paste it.
	assert.Contains(t, status.String(), server.URL+"/v1/oauth/authorize?")
}

func TestLoginReportsDeniedConsent(t *testing.T) {
	auth := &authServer{t: t}
	auth.consent = func(redirect *url.URL, authorize url.Values) {
		query := redirect.Query()
		query.Set("error", "access_denied")
		query.Set("error_description", "the user denied the request")
		query.Set("state", authorize.Get("state"))
		redirect.RawQuery = query.Encode()
	}
	server := httptest.NewServer(auth.handler())
	defer server.Close()

	_, err := Login(context.Background(), Options{
		APIBaseURL:  server.URL,
		Status:      &bytes.Buffer{},
		OpenBrowser: fakeBrowser(t),
	})

	var oauthErr *Error
	require.ErrorAs(t, err, &oauthErr)
	assert.Equal(t, "access_denied", oauthErr.Code)
	assert.Nil(t, auth.tokenReq, "no token request should be made after a denial")
}

func TestLoginRejectsStateMismatchFromServer(t *testing.T) {
	auth := &authServer{t: t}
	auth.consent = func(redirect *url.URL, _ url.Values) {
		query := redirect.Query()
		query.Set("code", "issued-code")
		query.Set("state", "not-the-state-we-sent")
		redirect.RawQuery = query.Encode()
	}
	server := httptest.NewServer(auth.handler())
	defer server.Close()

	_, err := Login(context.Background(), Options{
		APIBaseURL:  server.URL,
		Status:      &bytes.Buffer{},
		OpenBrowser: fakeBrowser(t),
	})

	require.ErrorContains(t, err, "state mismatch")
	assert.Nil(t, auth.tokenReq)
}

func TestLoginSurfacesTokenEndpointError(t *testing.T) {
	auth := &authServer{t: t}
	auth.token = func(w http.ResponseWriter) {
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error":             "invalid_grant",
			"error_description": "PKCE verification failed",
		})
	}
	server := httptest.NewServer(auth.handler())
	defer server.Close()

	_, err := Login(context.Background(), Options{
		APIBaseURL:  server.URL,
		Status:      &bytes.Buffer{},
		OpenBrowser: fakeBrowser(t),
	})

	require.EqualError(t, err, "token exchange failed: invalid_grant: PKCE verification failed")
	var oauthErr *Error
	require.ErrorAs(t, err, &oauthErr)
	assert.Equal(t, "invalid_grant", oauthErr.Code)
}

func TestLoginContinuesWhenBrowserCannotOpen(t *testing.T) {
	auth := &authServer{t: t}
	server := httptest.NewServer(auth.handler())
	defer server.Close()

	var status bytes.Buffer
	browser := fakeBrowser(t)
	token, err := Login(context.Background(), Options{
		APIBaseURL: server.URL,
		Status:     &status,
		OpenBrowser: func(address string) error {
			_ = browser(address)
			return assert.AnError
		},
	})

	require.NoError(t, err)
	assert.Equal(t, "access-token", token.AccessToken)
	assert.Contains(t, status.String(), "Could not open a browser")
}

func TestLoginRequiresAPIBaseURL(t *testing.T) {
	_, err := Login(context.Background(), Options{})

	require.EqualError(t, err, "api base url is required")
}

func TestExchangeCodeRejectsNonBearerToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "x", "token_type": "MAC"})
	}))
	defer server.Close()

	_, err := exchangeCode(context.Background(), server.Client(), server.URL, "code", "http://127.0.0.1:1/callback", "verifier")

	require.ErrorContains(t, err, `unsupported token_type "MAC"`)
}

func TestExchangeCodeFallsBackToStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("upstream exploded"))
	}))
	defer server.Close()

	_, err := exchangeCode(context.Background(), server.Client(), server.URL, "code", "http://127.0.0.1:1/callback", "verifier")

	require.EqualError(t, err, "token exchange failed: status code not ok: 500")
}
