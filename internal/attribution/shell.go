package attribution

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/requestyai/cli/internal/config"
	"github.com/requestyai/cli/internal/fileio"
)

const (
	// markerStart and markerEnd delimit the block the CLI owns in a startup
	// file, so it can update or remove its own lines and leave the rest of the
	// file alone.
	markerStart = "# --- Requesty attribution ---"
	markerEnd   = "# --- End Requesty attribution ---"

	// hookDir holds the hook itself, which the startup file only sources. The
	// hook can then grow without the startup file changing again.
	hookDir = "shell"

	hookPerm = 0o600
	// startupFilePerm applies to a startup file the user does not have yet.
	startupFilePerm = 0o644
)

// shell is an interactive shell the hook supports. The two flavours differ only
// in syntax: both export the same variables from the same commands.
type shell struct {
	// name is the executable the user's SHELL points at.
	name string
	// startupFile is read for every interactive session, relative to the home
	// directory.
	startupFile string
	// hookFile is where the CLI writes the hook, relative to its own directory.
	hookFile string
	// hook renders the hook body, and source renders the line loading it.
	hook   func(Set) string
	source func(hookPath string) string
}

var shells = []shell{
	{
		name:        "zsh",
		startupFile: ".zshrc",
		hookFile:    "attribution.sh",
		hook:        posixHook,
		source:      posixSource,
	},
	{
		name:        "bash",
		startupFile: ".bashrc",
		hookFile:    "attribution.sh",
		hook:        posixHook,
		source:      posixSource,
	},
	{
		name:        "fish",
		startupFile: filepath.Join(".config", "fish", "config.fish"),
		hookFile:    "attribution.fish",
		hook:        fishHook,
		source:      fishSource,
	},
}

// ShellHook is where the CLI put the hook and which startup file loads it, so a
// caller can tell the user what changed.
type ShellHook struct {
	HookPath        string
	StartupFilePath string
}

// InstallShellHook writes the hook that exports the dynamic dimensions and makes
// the user's shell load it. Harnesses that read header values from the
// environment need it; Pi does not, and neither does a harness configured
// without attribution.
//
// It is idempotent: installing again rewrites the hook and leaves one block
// behind in the startup file.
func InstallShellHook(set Set) (ShellHook, error) {
	shell, paths, err := resolveShellHook()
	if err != nil {
		return ShellHook{}, err
	}

	if err := fileio.Write(paths.HookPath, []byte(shell.hook(set)), hookPerm); err != nil {
		return ShellHook{}, fmt.Errorf("failed to write hook: %w", err)
	}

	block := fmt.Sprintf("%s\n%s\n%s\n", markerStart, shell.source(paths.HookPath), markerEnd)
	if err := rewriteStartupFile(paths.StartupFilePath, func(content string) string {
		return withBlock(content, block)
	}); err != nil {
		return ShellHook{}, err
	}

	return paths, nil
}

// RemoveShellHook takes the startup file back to how it read before the hook was
// installed and deletes the hook itself.
func RemoveShellHook() error {
	_, paths, err := resolveShellHook()
	if err != nil {
		return err
	}

	if err := rewriteStartupFile(paths.StartupFilePath, withoutBlock); err != nil {
		return err
	}

	if err := os.Remove(paths.HookPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove hook: %w", err)
	}

	return nil
}

// ShellHookInstalled reports whether the startup file loads the hook, which is
// how the CLI knows the user opted in on an earlier run.
func ShellHookInstalled() (bool, error) {
	_, paths, err := resolveShellHook()
	if err != nil {
		return false, err
	}

	content, err := os.ReadFile(paths.StartupFilePath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to read startup file: %w", err)
	}

	return strings.Contains(string(content), markerStart), nil
}

// resolveShellHook picks the shell from the environment and works out where its
// files live.
func resolveShellHook() (shell, ShellHook, error) {
	name := filepath.Base(os.Getenv("SHELL"))
	index := slices.IndexFunc(shells, func(candidate shell) bool {
		return candidate.name == name
	})
	if index < 0 {
		return shell{}, ShellHook{}, fmt.Errorf(
			"unsupported shell %q: attribution needs zsh, bash or fish", name)
	}
	current := shells[index]

	home, err := os.UserHomeDir()
	if err != nil {
		return shell{}, ShellHook{}, fmt.Errorf("failed to find home directory: %w", err)
	}

	dir, err := config.Dir()
	if err != nil {
		return shell{}, ShellHook{}, err
	}

	return current, ShellHook{
		HookPath:        filepath.Join(dir, hookDir, current.hookFile),
		StartupFilePath: filepath.Join(home, current.startupFile),
	}, nil
}

// rewriteStartupFile applies rewrite to a file the user owns, keeping the mode it
// already has and taking one backup before the first change.
func rewriteStartupFile(path string, rewrite func(content string) string) error {
	content, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to read startup file: %w", err)
	}

	perm, err := fileio.Mode(path, startupFilePerm)
	if err != nil {
		return err
	}

	rewritten := rewrite(string(content))
	if rewritten == string(content) {
		return nil
	}

	if err := fileio.BackupAndWrite(path, []byte(rewritten), perm); err != nil {
		return fmt.Errorf("failed to write startup file: %w", err)
	}

	return nil
}

// withBlock replaces the CLI's block, or appends it when there is none yet.
func withBlock(content, block string) string {
	content = strings.TrimRight(withoutBlock(content), "\n")
	if content != "" {
		content += "\n\n"
	}

	return content + block
}

// withoutBlock drops the CLI's block, and the blank line ahead of it, so
// installing and removing repeatedly cannot grow the file.
func withoutBlock(content string) string {
	lines := strings.Split(content, "\n")
	kept := make([]string, 0, len(lines))

	inside := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == markerStart:
			inside = true
			kept = trimTrailingBlankLines(kept)
		case inside:
			inside = trimmed != markerEnd
		default:
			kept = append(kept, line)
		}
	}

	return strings.Join(kept, "\n")
}

func trimTrailingBlankLines(lines []string) []string {
	for len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}

	return lines
}

func posixSource(hookPath string) string {
	return fmt.Sprintf("[ -r %q ] && . %q", hookPath, hookPath)
}

func fishSource(hookPath string) string {
	return fmt.Sprintf("test -r %q && source %q", hookPath, hookPath)
}

// posixHook renders the hook for zsh and bash. It runs before every prompt
// rather than once per session because the branch changes under a shell that
// never leaves the directory.
func posixHook(set Set) string {
	var hook strings.Builder

	hook.WriteString(hookHeader("#"))
	hook.WriteString("\n__requesty_attribution() {\n")
	for _, dimension := range set.dynamic() {
		fmt.Fprintf(&hook, "\texport %s=$(%s)\n", dimension.Env, dimension.Shell)
	}
	// The header list has to carry real newlines, which a shell keeps inside
	// double quotes, and reach the shell unexpanded, which is why it is not
	// quoted as a Go string.
	fmt.Fprintf(&hook, "\texport ANTHROPIC_CUSTOM_HEADERS=\"%s\"\n}\n", set.customHeaders(shellVar))

	hook.WriteString(`
# zsh runs precmd hooks before each prompt, bash runs PROMPT_COMMAND.
if [ -n "$ZSH_VERSION" ]; then
	autoload -Uz add-zsh-hook
	add-zsh-hook precmd __requesty_attribution
elif [ -n "$BASH_VERSION" ]; then
	case ";$PROMPT_COMMAND;" in
		*";__requesty_attribution;"*) ;;
		*) PROMPT_COMMAND="__requesty_attribution${PROMPT_COMMAND:+;$PROMPT_COMMAND}" ;;
	esac
fi

__requesty_attribution
`)

	return hook.String()
}

// fishHook renders the hook for fish, which reaches the shared commands through
// sh so there is one definition of how a dimension is read.
func fishHook(set Set) string {
	var hook strings.Builder

	hook.WriteString(hookHeader("#"))
	hook.WriteString("\n# fish emits fish_prompt before each prompt, and the branch can change\n" +
		"# without the directory changing.\n" +
		"function __requesty_attribution --on-event fish_prompt\n")
	for _, dimension := range set.dynamic() {
		fmt.Fprintf(&hook, "\tset -gx %s (sh -c '%s')\n", dimension.Env, dimension.Shell)
	}
	fmt.Fprintf(&hook, "\tset -gx ANTHROPIC_CUSTOM_HEADERS \"%s\"\nend\n\n__requesty_attribution\n",
		set.customHeaders(shellVar))

	return hook.String()
}

func hookHeader(comment string) string {
	return fmt.Sprintf(`%[1]s Requesty request attribution. Written by the Requesty CLI: change it with
%[1]s the CLI rather than by hand, since installing again replaces this file.
%[1]s
%[1]s It exports the repository and branch of the current directory so the
%[1]s harnesses routing through Requesty can send them as X-Requesty-* headers.
%[1]s Requesty strips those headers before forwarding a request to a provider.
`, comment)
}

// shellVar reads a dynamic dimension from the variable the hook exports, which
// is how the hook builds the header list Claude Code reads.
func shellVar(dimension Dimension) string {
	return "$" + dimension.Env
}

// customHeaders renders the header list Claude Code takes from
// ANTHROPIC_CUSTOM_HEADERS, one "Name: value" pair per line.
func (s Set) customHeaders(reference Reference) string {
	lines := make([]string, 0, len(s))
	for _, dimension := range s {
		lines = append(lines, dimension.Header+": "+dimension.spell(reference))
	}

	return strings.Join(lines, "\n")
}

// dynamic returns the dimensions the hook has to read on every prompt.
func (s Set) dynamic() Set {
	dimensions := make(Set, 0, len(s))
	for _, dimension := range s {
		if dimension.Dynamic() {
			dimensions = append(dimensions, dimension)
		}
	}

	return dimensions
}
