#!/bin/sh
#
# Requesty CLI installer.
#
#   curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh
#
# Re-run the same command to upgrade. The binary is replaced in place and
# everything under $REQUESTY_HOME (including config.json) is left untouched.
#
# Environment variables:
#   REQUESTY_HOME         Data directory (default: $HOME/.requesty)
#   REQUESTY_INSTALL_DIR  Directory for the binary (default: $REQUESTY_HOME/bin)
#   REQUESTY_VERSION      Release tag to install (default: latest)
#   NO_COLOR              Disable colored output
#
# A marker file is created in the temporary directory while installing and
# removed once the installation completes, so an interrupted run leaves a trace.
#
# Flags:
#   --version <tag>    Install a specific release, e.g. --version v1.2.3
#   --install-dir <d>  Install the binary into <d>
#   --no-modify-path   Do not touch shell configuration files
#   --force            Reinstall even when the target version is installed
#
# The script is wrapped in a function so a truncated download cannot execute
# a partial script.
{

REPO="requestyai/cli"
BINARY="requesty"
MARKER_BEGIN="# >>> requesty cli installer >>>"
MARKER_END="# <<< requesty cli installer <<<"

# Colors are used when stdout is a terminal and NO_COLOR is unset. The script
# is normally piped into sh, but stdout still points at the terminal.
setup_style() {
	BOLD="" DIM="" RED="" GREEN="" YELLOW="" CYAN="" RESET=""
	if [ -t 1 ] && [ -z "${NO_COLOR:-}" ] && [ "${TERM:-dumb}" != "dumb" ]; then
		BOLD="$(printf '\033[1m')"
		DIM="$(printf '\033[2m')"
		RED="$(printf '\033[31m')"
		GREEN="$(printf '\033[32m')"
		YELLOW="$(printf '\033[33m')"
		CYAN="$(printf '\033[36m')"
		RESET="$(printf '\033[0m')"
	fi

	TICK="*" CROSS="x" BAR="|"
	case "${LC_ALL:-${LC_CTYPE:-${LANG:-}}}" in
	*UTF-8* | *utf8* | *UTF8* | *utf-8*)
		TICK="✓" CROSS="✗" BAR="│"
		;;
	esac
}

banner() {
	printf '%s' "$CYAN"
	cat <<'EOF'

  ____                            _
 |  _ \ ___  __ _ _   _  ___  ___| |_ _   _
 | |_) / _ \/ _` | | | |/ _ \/ __| __| | | |
 |  _ <  __/ (_| | |_| |  __/\__ \ |_| |_| |
 |_| \_\___|\__, |\__,_|\___||___/\__|\__, |
               |_|                    |___/
EOF
	printf '%s\n' "$RESET"
	printf '  %sRequesty CLI installer%s\n\n' "$BOLD" "$RESET"
}

# step prints a completed installation step.
step() {
	printf '  %s%s%s %s\n' "$GREEN" "$TICK" "$RESET" "$1"
}

warn() {
	printf '  %s!%s %s\n' "$YELLOW" "$RESET" "$1"
}

info() {
	printf '  %s\n' "$1"
}

# action prints one line inside the highlighted "next steps" block.
action() {
	printf '  %s%s%s  %s\n' "$CYAN" "$BAR" "$RESET" "$1"
}

command_hint() {
	printf '%s%s%s' "$BOLD" "$1" "$RESET"
}

fail() {
	printf '\n  %s%s error:%s %s\n' "$RED" "$CROSS" "$RESET" "$1" >&2

	if [ -f "${MARKER_FILE:-}" ]; then
		printf '  %sthe installation did not complete, %s was left behind%s\n' "$DIM" "$MARKER_FILE" "$RESET" >&2
	fi

	exit 1
}

require_command() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is required"
}

usage() {
	cat <<EOF
Install or upgrade the Requesty CLI.

Usage: install.sh [options]

Options:
  --version <tag>      Install a specific release, e.g. --version v1.2.3
  --install-dir <dir>  Install the binary into <dir>
  --no-modify-path     Do not touch shell configuration files
  --force              Reinstall even when the target version is installed
  -h, --help           Show this help
EOF
}

detect_os() {
	os="$(uname -s)"
	case "$os" in
	Darwin) echo "darwin" ;;
	Linux) echo "linux" ;;
	*) fail "unsupported operating system: $os. Windows binaries are available on the releases page: https://github.com/$REPO/releases/latest" ;;
	esac
}

detect_arch() {
	arch="$(uname -m)"
	case "$arch" in
	x86_64 | amd64) echo "amd64" ;;
	arm64 | aarch64) echo "arm64" ;;
	*) fail "unsupported architecture: $arch" ;;
	esac
}

download() {
	url="$1"
	output="$2"

	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$url" -o "$output" || fail "failed to download $url"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$output" "$url" || fail "failed to download $url"
	else
		fail "curl or wget is required"
	fi
}

verify_checksum() {
	dir="$1"
	file="$2"

	if command -v sha256sum >/dev/null 2>&1; then
		checksum="$(sha256sum "$dir/$file" | cut -d ' ' -f 1)"
	elif command -v shasum >/dev/null 2>&1; then
		checksum="$(shasum -a 256 "$dir/$file" | cut -d ' ' -f 1)"
	else
		warn "Neither sha256sum nor shasum is available, skipping checksum verification"
		return 0
	fi

	expected="$(grep " $file\$" "$dir/checksums.txt" | cut -d ' ' -f 1)"
	[ -n "$expected" ] || fail "$file is missing from checksums.txt"
	[ "$checksum" = "$expected" ] || fail "checksum mismatch for $file: expected $expected, got $checksum"
	step "Verified checksum"
}

# latest_version resolves the tag the "latest" release points at, so the
# installed version can be recorded and compared on the next run.
latest_version() {
	url="https://api.github.com/repos/$REPO/releases/latest"
	tag="$(download "$url" - | sed -n 's/.*"tag_name"[ ]*:[ ]*"\([^"]*\)".*/\1/p' | head -n 1)"
	[ -n "$tag" ] || fail "could not determine the latest release of $REPO"

	echo "$tag"
}

installed_version() {
	[ -f "$RECEIPT" ] || return 0
	head -n 1 "$RECEIPT"
}

install_release() {
	archive="${BINARY}_${OS}_${ARCH}.tar.gz"
	base="https://github.com/$REPO/releases/download/$VERSION"

	tmp="$(mktemp -d)"
	trap 'rm -rf "$tmp"' EXIT INT TERM

	download "$base/$archive" "$tmp/$archive"
	download "$base/checksums.txt" "$tmp/checksums.txt"
	step "Downloaded $archive"
	verify_checksum "$tmp" "$archive"

	tar -xzf "$tmp/$archive" -C "$tmp"
	[ -f "$tmp/$BINARY" ] || fail "the archive does not contain a $BINARY binary"

	mkdir -p "$INSTALL_DIR"
	chmod 755 "$tmp/$BINARY"

	# Replace via rename so a running binary is never written into.
	mv -f "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"
	step "Installed $BINARY to $INSTALL_DIR/$BINARY"

	mkdir -p "$REQUESTY_HOME"
}

# shell_profile prints the configuration file of the current shell.
shell_profile() {
	case "$1" in
	fish) echo "${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
	zsh) echo "${ZDOTDIR:-$HOME}/.zshrc" ;;
	bash)
		if [ ! -f "$HOME/.bashrc" ] && [ -f "$HOME/.bash_profile" ]; then
			echo "$HOME/.bash_profile"
		else
			echo "$HOME/.bashrc"
		fi
		;;
	*) echo "$HOME/.profile" ;;
	esac
}

# path_line prints the line that puts the install directory on PATH.
path_line() {
	case "$1" in
	fish) echo "fish_add_path \"$INSTALL_DIR\"" ;;
	*) echo "export PATH=\"$INSTALL_DIR:\$PATH\"" ;;
	esac
}

# configure_path appends the PATH line to the profile inside a marker block so
# upgrades stay idempotent. It returns 0 only when the profile was modified.
configure_path() {
	profile="$1"
	line="$2"

	[ -f "$profile" ] || return 1
	grep -F "$MARKER_BEGIN" "$profile" >/dev/null 2>&1 && return 1

	{
		echo ""
		echo "$MARKER_BEGIN"
		echo "$line"
		echo "$MARKER_END"
	} >>"$profile"
}

# short_path replaces a leading $HOME with ~ so commands are easier to read.
short_path() {
	case "$1" in
	"$HOME"/*) printf '~%s\n' "${1#"$HOME"}" ;;
	*) printf '%s\n' "$1" ;;
	esac
}

# summary prints the final report. The optional pair of arguments is a PATH
# related instruction and the command that goes with it.
summary() {
	path_instruction="${1:-}"
	path_command="${2:-}"

	printf '\n  %s%s%s %s%s %s is ready%s\n\n' "$GREEN" "$TICK" "$RESET" "$BOLD" "$BINARY" "$VERSION" "$RESET"

	if [ -f "$REQUESTY_HOME/config.json" ]; then
		info "${DIM}Kept your configuration in $REQUESTY_HOME/config.json${RESET}"
		printf '\n'
	fi

	printf '  %s%s%s  %sNext steps%s\n' "$CYAN" "$BAR" "$RESET" "$BOLD" "$RESET"
	action ""

	n=1
	if [ -n "$path_instruction" ]; then
		action "$n. $path_instruction"
		action "   $(command_hint "$path_command")"
		n=$((n + 1))
	fi

	action "$n. Start the CLI:"
	action "   $(command_hint "$BINARY")"
	action ""
	printf '\n'
}

# finish handles PATH configuration and prints the summary.
finish() {
	rm -f "$MARKER_FILE"

	if [ "$MODIFY_PATH" != true ]; then
		summary
		return 0
	fi

	case ":$PATH:" in
	*":$INSTALL_DIR:"*)
		summary
		return 0
		;;
	esac

	shell_name="$(basename "${SHELL:-sh}")"
	profile="$(shell_profile "$shell_name")"
	line="$(path_line "$shell_name")"

	if [ ! -f "$profile" ]; then
		warn "Could not configure PATH because $profile does not exist"
		summary "Add this line to your shell configuration:" "$line"
	elif configure_path "$profile" "$line"; then
		step "Added $INSTALL_DIR to PATH in $profile"
		summary "Reload your shell (or open a new terminal):" "source $(short_path "$profile")"
	else
		summary "Reload your shell (or open a new terminal):" "source $(short_path "$profile")"
	fi
}

main() {
	set -eu

	REQUESTY_HOME="${REQUESTY_HOME:-$HOME/.requesty}"
	INSTALL_DIR="${REQUESTY_INSTALL_DIR:-$REQUESTY_HOME/bin}"
	VERSION="${REQUESTY_VERSION:-latest}"
	MODIFY_PATH=true
	FORCE=false

	while [ $# -gt 0 ]; do
		case "$1" in
		--version)
			VERSION="${2:-}"
			[ -n "$VERSION" ] || fail "--version requires a release tag"
			shift 2
			;;
		--install-dir)
			INSTALL_DIR="${2:-}"
			[ -n "$INSTALL_DIR" ] || fail "--install-dir requires a directory"
			shift 2
			;;
		--no-modify-path)
			MODIFY_PATH=false
			shift
			;;
		--force)
			FORCE=true
			shift
			;;
		--help | -h)
			usage
			exit 0
			;;
		*)
			fail "unknown argument: $1"
			;;
		esac
	done

	setup_style
	banner

	require_command uname
	require_command tar

	RECEIPT="$REQUESTY_HOME/version"
	OS="$(detect_os)"
	ARCH="$(detect_arch)"
	step "Detected $OS/$ARCH"

	# A leftover marker file is how a user, or we, can tell that a previous run
	# died halfway through.
	MARKER_FILE="${TMPDIR:-/tmp}/requesty-cli-install.$(date +%Y%m%d%H%M%S).$$"
	: >"$MARKER_FILE"

	if [ "$VERSION" = "latest" ]; then
		VERSION="$(latest_version)"
		step "Resolved latest release: $VERSION"
	fi

	installed="$(installed_version)"
	if [ "$FORCE" = false ] && [ "$installed" = "$VERSION" ] && [ -x "$INSTALL_DIR/$BINARY" ]; then
		step "$BINARY $VERSION is already installed in $INSTALL_DIR"
		finish
		exit 0
	fi

	if [ -n "$installed" ]; then
		info "${DIM}Upgrading $BINARY from $installed to $VERSION${RESET}"
	else
		info "${DIM}Installing $BINARY $VERSION${RESET}"
	fi

	install_release
	printf '%s\n' "$VERSION" >"$RECEIPT"

	finish
}

main "$@"

}
