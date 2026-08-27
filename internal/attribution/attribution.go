// Package attribution builds the request attribution headers a harness sends to
// Requesty, so spend can be grouped by the repository and branch a request came
// from and by the person who made it.
//
// Requesty turns any X-Requesty-<Name> request header into an extra.<Name>
// dimension on the request, which the usage APIs can group by, and strips the
// header before forwarding the request to a provider. The values reach Requesty
// and nothing else.
//
// Attribution is off unless the user asks for it: the zero Set writes no
// headers, so a harness configured without it sends exactly what it sent before.
package attribution

import (
	"fmt"
	"os/user"
	"strings"
)

const (
	// The headers below arrive at Requesty as extra.Repo, extra.Branch and
	// extra.User.
	headerRepo   = "X-Requesty-Repo"
	headerBranch = "X-Requesty-Branch"
	headerUser   = "X-Requesty-User"

	// The variables below are the ones the shell hook exports for harnesses
	// that read header values from the environment.
	envRepo   = "REQUESTY_REPO"
	envBranch = "REQUESTY_BRANCH"

	// unknown stands in for a value the shell could not read, so a harness
	// never has to send, or refuse to send, an empty header.
	unknown = "none"
)

// The commands below are written in POSIX shell, and always print a value on a
// successful exit, so the shell hook, the fish hook and Pi can share one
// definition of how a dimension is read. They must stay free of single quotes:
// the fish hook passes them to `sh -c '…'`.
const (
	// repoShell prints owner/name for the repository holding the working
	// directory, dropping the transport, any credentials, the host and the .git
	// suffix from the origin remote.
	repoShell = `url=$(git remote get-url origin 2>/dev/null); url=${url%.git}; url=${url#*://}; url=${url#*@}; url=${url#*[:/]}; echo "${url:-` + unknown + `}"`

	// branchShell prints the checked out branch, and falls back when HEAD is
	// detached or the directory is not a repository at all.
	branchShell = `branch=$(git symbolic-ref --quiet --short HEAD 2>/dev/null); echo "${branch:-` + unknown + `}"`
)

// Dimension is one thing a request can be attributed to, carried in one header.
//
// A dimension is either static, and holds the Value the CLI resolved while
// configuring, or dynamic, and holds the Env and Shell a harness needs to read
// the value itself on every launch. The repository and branch have to be
// dynamic: the CLI is configured once, from one directory, while harnesses run
// later from anywhere, so a value baked in now would be confidently wrong.
type Dimension struct {
	Header string
	Value  string
	Env    string
	Shell  string
}

// Dynamic reports whether the value depends on where a harness runs, and so
// cannot be resolved while configuring.
func (d Dimension) Dynamic() bool {
	return d.Env != ""
}

// spell returns the value to write for the dimension, asking reference for the
// dynamic ones.
func (d Dimension) spell(reference Reference) string {
	if d.Dynamic() {
		return reference(d)
	}

	return d.Value
}

// Reference spells a dynamic dimension the way one harness reads a value it
// cannot be given upfront. Harnesses differ: Codex names an environment
// variable, OpenCode expands an {env:…} placeholder, and Pi runs a command.
type Reference func(Dimension) string

// EnvName names the environment variable, for Codex, whose env_http_headers
// table holds variable names rather than values.
func EnvName(dimension Dimension) string {
	return dimension.Env
}

// EnvPlaceholder wraps the environment variable in the {env:…} placeholder
// OpenCode expands wherever it appears in its config file.
func EnvPlaceholder(dimension Dimension) string {
	return fmt.Sprintf("{env:%s}", dimension.Env)
}

// ShellCommand asks Pi to run the command itself on every launch. Pi treats a
// header it cannot resolve as a configuration error and refuses to start, so it
// reads the repository and branch directly instead of relying on a shell that
// may never have loaded the hook.
func ShellCommand(dimension Dimension) string {
	return "!" + dimension.Shell
}

// Set is the dimensions to attribute requests by. A nil Set means the user did
// not opt in, which is the default.
type Set []Dimension

// New builds the dimensions for this machine.
func New() (Set, error) {
	name, err := currentUser()
	if err != nil {
		return nil, err
	}

	return Set{
		{Header: headerRepo, Env: envRepo, Shell: repoShell},
		{Header: headerBranch, Env: envBranch, Shell: branchShell},
		{Header: headerUser, Value: name},
	}, nil
}

// Static adds the dimensions the CLI already resolved to headers, for harnesses
// that can only be given fixed values.
func (s Set) Static(headers map[string]string) map[string]string {
	for _, dimension := range s {
		if !dimension.Dynamic() {
			headers = put(headers, dimension.Header, dimension.Value)
		}
	}

	return headers
}

// Dynamic returns the dimensions a harness has to read on every launch, spelled
// with reference. Codex keeps those apart from its fixed headers, so they come
// back on their own.
func (s Set) Dynamic(reference Reference) map[string]string {
	var headers map[string]string
	for _, dimension := range s {
		if dimension.Dynamic() {
			headers = put(headers, dimension.Header, reference(dimension))
		}
	}

	return headers
}

// All adds every dimension to headers, spelling the dynamic ones with
// reference, for harnesses that hold resolved and unresolved values together.
func (s Set) All(headers map[string]string, reference Reference) map[string]string {
	for _, dimension := range s {
		headers = put(headers, dimension.Header, dimension.spell(reference))
	}

	return headers
}

// currentUser names the person a harness runs as. It is resolved while
// configuring because, unlike the repository and branch, it does not change with
// the directory a harness runs from.
func currentUser() (string, error) {
	current, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("failed to resolve current user: %w", err)
	}

	return sanitize(current.Username), nil
}

// sanitize keeps only the characters a header value may safely carry, so a user
// name can never break the header list the shell hook writes.
func sanitize(value string) string {
	var sanitized strings.Builder
	for _, char := range value {
		switch {
		case char >= 'a' && char <= 'z',
			char >= 'A' && char <= 'Z',
			char >= '0' && char <= '9',
			char == '.', char == '_', char == '-', char == '@':
			sanitized.WriteRune(char)
		default:
			sanitized.WriteRune('-')
		}
	}

	if sanitized.Len() == 0 {
		return unknown
	}

	return sanitized.String()
}

// put adds a header, allocating the map when the caller had none to add to.
func put(headers map[string]string, name, value string) map[string]string {
	if headers == nil {
		headers = make(map[string]string, 1)
	}
	headers[name] = value

	return headers
}
