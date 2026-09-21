package oauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	// ClientID identifies the CLI to the authorization server. It is a public
	// client: there is no secret, and PKCE binds the code to this process.
	ClientID = "requesty-cli"

	// Scope is everything the CLI asks for: enough to list groups and create
	// an API key in one of them.
	Scope = "manage:group:r manage:apikey:w"

	authorizePath = "/v1/oauth/authorize"
	tokenPath     = "/v1/oauth/token"
	resourcePath  = "/v1/manage"
)

// Token is a short-lived credential for the management API. The refresh token
// the server also returns is deliberately dropped: the CLI provisions an API
// key within the access token's lifetime and never needs to renew it.
type Token struct {
	AccessToken string
	TokenType   string
	Scope       string
	ExpiresIn   time.Duration
}

// Scopes splits the granted scope into its individual permissions.
func (t *Token) Scopes() []string {
	return strings.Fields(t.Scope)
}

// Options configures Login. Only APIBaseURL is required; the rest exist so
// tests can stand in for the browser, the terminal, and the network.
type Options struct {
	// APIBaseURL is the management API address, for example
	// https://api-v2.requesty.ai, which also hosts the authorization server.
	APIBaseURL string

	// Status receives progress messages, including the URL to visit for users
	// whose browser cannot be opened from the terminal. Defaults to stderr.
	Status io.Writer

	// OpenBrowser opens url in the user's browser. Defaults to the platform
	// launcher. A failure is reported on Status but does not abort the login.
	OpenBrowser func(url string) error

	// OnAuthorizeURL, when set, is told the address the browser is sent to,
	// for front ends that draw it themselves rather than reading Status.
	OnAuthorizeURL func(url string)

	// HTTPClient sends the token request. Defaults to one with a timeout.
	HTTPClient *http.Client
}

// Login runs the authorization code flow end to end: it starts a loopback
// listener, sends the user to the consent page, waits for the redirect, and
// exchanges the code for a token.
func Login(ctx context.Context, opts Options) (*Token, error) {
	apiBaseURL := strings.TrimRight(opts.APIBaseURL, "/")
	if apiBaseURL == "" {
		return nil, errors.New("api base url is required")
	}

	status := opts.Status
	if status == nil {
		status = os.Stderr
	}
	open := opts.OpenBrowser
	if open == nil {
		open = openBrowser
	}
	httpClient := opts.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 30 * time.Second}
	}

	verifier, err := NewVerifier()
	if err != nil {
		return nil, err
	}
	state, err := NewState()
	if err != nil {
		return nil, err
	}

	server, err := listenCallback(state)
	if err != nil {
		return nil, err
	}
	defer server.Close()

	redirectURI := server.RedirectURI()
	authorizeURL := authorizeURL(apiBaseURL, redirectURI, state, Challenge(verifier))
	if opts.OnAuthorizeURL != nil {
		opts.OnAuthorizeURL(authorizeURL)
	}

	_, _ = fmt.Fprintf(status,
		"Opening your browser to sign in to Requesty.\n\nIf it does not open, visit this address:\n\n  %s\n\n",
		authorizeURL)
	if err := open(authorizeURL); err != nil {
		_, _ = fmt.Fprintf(status, "Could not open a browser (%v); paste the address above into one.\n\n", err)
	}
	_, _ = fmt.Fprintln(status, "Waiting for you to finish signing in...")

	code, err := server.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("sign-in was not completed: %w", err)
	}

	return exchangeCode(ctx, httpClient, apiBaseURL, code, redirectURI, verifier)
}

// authorizeURL builds the address the browser visits to start the flow.
func authorizeURL(apiBaseURL, redirectURI, state, challenge string) string {
	query := url.Values{
		"response_type":         {"code"},
		"client_id":             {ClientID},
		"redirect_uri":          {redirectURI},
		"scope":                 {Scope},
		"resource":              {apiBaseURL + resourcePath},
		"state":                 {state},
		"code_challenge":        {challenge},
		"code_challenge_method": {"S256"},
	}

	return apiBaseURL + authorizePath + "?" + query.Encode()
}

// exchangeCode redeems an authorization code at the token endpoint. The
// request is form-encoded, as the OAuth token endpoint requires, and the
// redirect URI must be byte-for-byte what the authorize request carried.
func exchangeCode(ctx context.Context, httpClient *http.Client, apiBaseURL, code, redirectURI, verifier string) (*Token, error) {
	form := url.Values{
		"grant_type":    {"authorization_code"},
		"code":          {code},
		"redirect_uri":  {redirectURI},
		"client_id":     {ClientID},
		"code_verifier": {verifier},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+tokenPath, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, tokenError(resp.StatusCode, body)
	}

	var payload struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		ExpiresIn   int64  `json:"expires_in"`
		Scope       string `json:"scope"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}
	if payload.AccessToken == "" {
		return nil, errors.New("token response has no access_token")
	}
	if !strings.EqualFold(payload.TokenType, "Bearer") {
		return nil, fmt.Errorf("token response has unsupported token_type %q", payload.TokenType)
	}

	return &Token{
		AccessToken: payload.AccessToken,
		TokenType:   payload.TokenType,
		Scope:       payload.Scope,
		ExpiresIn:   time.Duration(payload.ExpiresIn) * time.Second,
	}, nil
}

// tokenError describes a rejected token request, preferring the server's
// error and error_description over the bare status code.
func tokenError(statusCode int, body []byte) error {
	var payload struct {
		Error       string `json:"error"`
		Description string `json:"error_description"`
	}

	if err := json.Unmarshal(body, &payload); err == nil && payload.Error != "" {
		return fmt.Errorf("token exchange failed: %w", &Error{Code: payload.Error, Description: payload.Description})
	}

	return fmt.Errorf("token exchange failed: status code not ok: %d", statusCode)
}
