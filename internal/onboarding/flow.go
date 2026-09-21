package onboarding

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/requestyai/cli/internal/client"
	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/oauth"
)

// APIKeysURL is where keys are managed once created.
const APIKeysURL = "https://app.requesty.ai/api-keys"

var (
	errGroupChoiceRequired = errors.New("choose a group for the API key")
	errGroupNotFound       = errors.New("group not found")
)

// Options configures onboarding. Only Config is needed.
type Options struct {
	// Config is the current settings; the new key is added to it.
	Config config.Config
	// GroupID picks the group for the key by id or name instead of asking.
	GroupID string
	// Harness is the display name of the harness that starts once the key is
	// saved, set only by `requesty <harness>`.
	Harness string
}

// defaultKeyName is what the created key is called, so it can be recognised on
// the API keys page: the CLI plus this machine's hostname.
func defaultKeyName() string {
	host, _ := os.Hostname()
	return keyNameFor(host)
}

type result struct {
	Config  config.Config
	KeyName string
	Group   *client.Group
}

// session is a browser sign-in that has not yet created a key.
type session struct {
	opts   Options
	api    *client.Client
	groups []client.Group
}

func signIn(ctx context.Context, opts Options, onAuthorizeURL func(string)) (*session, error) {
	var status io.Writer = os.Stderr
	if onAuthorizeURL != nil {
		// The interactive UI owns progress output and displays the URL itself.
		status = io.Discard
	}
	token, err := oauth.Login(ctx, oauth.Options{
		APIBaseURL:     opts.Config.APIBaseURL(),
		Status:         status,
		OnAuthorizeURL: onAuthorizeURL,
	})
	if err != nil {
		return nil, err
	}

	authConfig := opts.Config
	authConfig.APIKey = token.AccessToken
	api := client.New(authConfig)
	groups, err := api.Groups(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list groups: %w", err)
	}

	return &session{opts: opts, api: api, groups: groups}, nil
}

func (s *session) chooseGroup() (*client.Group, error) {
	if s.opts.GroupID != "" {
		group, ok := findGroup(s.groups, s.opts.GroupID)
		if !ok {
			return nil, fmt.Errorf("%w: %q is not one of your groups", errGroupNotFound, s.opts.GroupID)
		}
		return group, nil
	}

	switch len(s.groups) {
	case 0:
		return nil, nil
	case 1:
		return &s.groups[0], nil
	default:
		return nil, fmt.Errorf("%w: you belong to %d groups (%s)",
			errGroupChoiceRequired, len(s.groups), strings.Join(groupNames(s.groups), ", "))
	}
}

func (s *session) createKey(ctx context.Context, group *client.Group) (result, error) {
	name := defaultKeyName()
	input := client.CreateAPIKeyInput{Name: name}
	if group != nil {
		input.GroupID = group.ID
	}
	created, err := s.api.CreateAPIKey(ctx, input)
	if err != nil {
		return result{}, fmt.Errorf("failed to create api key: %w", err)
	}

	cfg := s.opts.Config
	cfg.APIKey = created.Secret
	if err := config.Save(cfg); err != nil {
		return result{}, fmt.Errorf("failed to save config: %w", err)
	}

	return result{Config: cfg, KeyName: name, Group: group}, nil
}

func provision(ctx context.Context, opts Options) (result, error) {
	s, err := signIn(ctx, opts, nil)
	if err != nil {
		return result{}, err
	}
	group, err := s.chooseGroup()
	if errors.Is(err, errGroupChoiceRequired) {
		return result{}, fmt.Errorf("%w; run `requesty login --group <name>` to pick one", err)
	}
	if err != nil {
		return result{}, err
	}
	return s.createKey(ctx, group)
}

func findGroup(groups []client.Group, idOrName string) (*client.Group, bool) {
	for i := range groups {
		if groups[i].ID == idOrName {
			return &groups[i], true
		}
	}
	for i := range groups {
		if strings.EqualFold(groups[i].Name, idOrName) {
			return &groups[i], true
		}
	}
	return nil, false
}

func groupNames(groups []client.Group) []string {
	names := make([]string, 0, len(groups))
	for _, group := range groups {
		names = append(names, group.Name)
	}
	return names
}

const (
	keyNamePrefix  = "Requesty CLI"
	keyNameHostMax = 40
)

func keyNameFor(host string) string {
	host, _, _ = strings.Cut(host, ".")
	host = strings.Map(func(r rune) rune {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '-', r == '_':
			return r
		default:
			return -1
		}
	}, host)
	if len(host) > keyNameHostMax {
		host = host[:keyNameHostMax]
	}
	if host == "" {
		return keyNamePrefix
	}
	return keyNamePrefix + " (" + host + ")"
}
