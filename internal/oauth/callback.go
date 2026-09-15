package oauth

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"
)

const (
	callbackPath = "/callback"

	// loginTimeout bounds how long the CLI waits for the user to finish in the
	// browser before giving up.
	loginTimeout = 5 * time.Minute

	// shutdownTimeout bounds how long Close waits for the browser's connection
	// to drain before dropping it.
	shutdownTimeout = 2 * time.Second
)

// Error is a failure the authorization server reported, either on the redirect
// back to the CLI or from the token endpoint.
type Error struct {
	Code        string
	Description string
}

func (e *Error) Error() string {
	if e.Description == "" {
		return e.Code
	}

	return fmt.Sprintf("%s: %s", e.Code, e.Description)
}

type callbackResult struct {
	code string
	err  error
}

// callbackServer is the loopback HTTP server the browser lands on once the
// user has approved or denied the request. It accepts exactly one outcome.
type callbackServer struct {
	listener net.Listener
	server   *http.Server
	state    string
	results  chan callbackResult
	once     sync.Once
}

// listenCallback starts serving on a free port of the IPv4 loopback interface.
// The redirect URI must use the 127.0.0.1 literal, not localhost, so that is
// what we bind to.
func listenCallback(state string) (*callbackServer, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("failed to listen for the browser callback: %w", err)
	}

	server := &callbackServer{
		listener: listener,
		state:    state,
		results:  make(chan callbackResult, 1),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET "+callbackPath, server.handle)
	server.server = &http.Server{
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		// Serve always returns a non-nil error; after Shutdown it is
		// ErrServerClosed, which is the expected way to stop.
		_ = server.server.Serve(listener)
	}()

	return server, nil
}

// RedirectURI is the address the authorization server sends the browser back
// to. The exact same string must be presented at the token endpoint.
func (s *callbackServer) RedirectURI() string {
	return "http://" + s.listener.Addr().String() + callbackPath
}

// Wait blocks until the browser delivers an outcome, the context ends, or the
// login timeout passes.
func (s *callbackServer) Wait(ctx context.Context) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, loginTimeout)
	defer cancel()

	select {
	case result := <-s.results:
		return result.code, result.err
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", fmt.Errorf("timed out waiting for the browser sign-in: %w", ctx.Err())
		}

		return "", ctx.Err()
	}
}

// Close stops the server, giving the browser a moment to read its response.
func (s *callbackServer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := s.server.Shutdown(ctx); err != nil {
		_ = s.server.Close()
	}
}

func (s *callbackServer) handle(w http.ResponseWriter, r *http.Request) {
	result := s.resolve(r.URL.Query())

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")

	if result.err != nil {
		w.WriteHeader(http.StatusBadRequest)
		writePage(w, callbackPage{
			Title:   "Sign-in failed",
			Message: "Could not sign in to Requesty CLI: " + result.err.Error() + ".",
			Hint:    "You can close this tab and return to the terminal.",
		})
	} else {
		writePage(w, callbackPage{
			Success: true,
			Title:   "Signed in",
			Message: "You're signed in to Requesty CLI. You can close this tab.",
			Hint:    "Return to the terminal to continue.",
		})
	}

	s.once.Do(func() { s.results <- result })
}

// resolve turns the authorization response into a code or an error. The state
// check comes first so that a response meant for another attempt, or forged
// by a page that guessed the port, is never acted on.
func (s *callbackServer) resolve(query url.Values) callbackResult {
	if subtle.ConstantTimeCompare([]byte(query.Get("state")), []byte(s.state)) != 1 {
		return callbackResult{err: errors.New("state mismatch: the response did not belong to this sign-in attempt")}
	}

	if code := query.Get("error"); code != "" {
		return callbackResult{err: &Error{Code: code, Description: query.Get("error_description")}}
	}

	code := query.Get("code")
	if code == "" {
		return callbackResult{err: errors.New("authorization response has no code")}
	}

	return callbackResult{code: code}
}
