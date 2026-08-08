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

info() {
	printf '%s\n' "$1"
}

fail() {
	printf 'error: %s\n' "$1" >&2

	if [ -f "${MARKER_FILE:-}" ]; then
		printf 'the installation did not complete, %s was left behind\n' "$MARKER_FILE" >&2
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
		info "Neither sha256sum nor shasum is available, skipping checksum verification"
		return 0
	fi

	expected="$(grep " $file\$" "$dir/checksums.txt" | cut -d ' ' -f 1)"
	[ -n "$expected" ] || fail "$file is missing from checksums.txt"
	[ "$checksum" = "$expected" ] || fail "checksum mismatch for $file: expected $expected, got $checksum"
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

	info "Downloading $archive"
	download "$base/$archive" "$tmp/$archive"
	download "$base/checksums.txt" "$tmp/checksums.txt"
	verify_checksum "$tmp" "$archive"

	tar -xzf "$tmp/$archive" -C "$tmp"
	[ -f "$tmp/$BINARY" ] || fail "the archive does not contain a $BINARY binary"

	mkdir -p "$INSTALL_DIR"
	chmod 755 "$tmp/$BINARY"

	# Replace via rename so a running binary is never written into.
	mv -f "$tmp/$BINARY" "$INSTALL_DIR/$BINARY"

	mkdir -p "$REQUESTY_HOME"
}

# configure_path adds the install directory to PATH in the configuration file
# of the current shell, inside a marker block so upgrades stay idempotent.
configure_path() {
	[ "$MODIFY_PATH" = true ] || return 0

	case ":$PATH:" in
	*":$INSTALL_DIR:"*) return 0 ;;
	esac

	shell_name="$(basename "${SHELL:-sh}")"
	case "$shell_name" in
	fish)
		profile="${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish"
		line="fish_add_path \"$INSTALL_DIR\""
		;;
	zsh)
		profile="${ZDOTDIR:-$HOME}/.zshrc"
		line="export PATH=\"$INSTALL_DIR:\$PATH\""
		;;
	bash)
		profile="$HOME/.bashrc"
		[ -f "$profile" ] || [ ! -f "$HOME/.bash_profile" ] || profile="$HOME/.bash_profile"
		line="export PATH=\"$INSTALL_DIR:\$PATH\""
		;;
	*)
		profile="$HOME/.profile"
		line="export PATH=\"$INSTALL_DIR:\$PATH\""
		;;
	esac

	if [ ! -f "$profile" ]; then
		info "Could not configure PATH because $profile does not exist. Add this line to your shell configuration:"
		info "  $line"
		return 0
	fi

	if grep -F "$MARKER_BEGIN" "$profile" >/dev/null 2>&1; then
		PROFILE="$profile"
		return 0
	fi

	{
		echo ""
		echo "$MARKER_BEGIN"
		echo "$line"
		echo "$MARKER_END"
	} >>"$profile"

	PROFILE="$profile"
	PROFILE_UPDATED=true
}

summary() {
	info "Installed $BINARY $VERSION to $INSTALL_DIR/$BINARY"

	if [ -f "$REQUESTY_HOME/config.json" ]; then
		info "Kept your configuration in $REQUESTY_HOME/config.json"
	fi

	if [ "${PROFILE_UPDATED:-false}" = true ]; then
		info "Added $INSTALL_DIR to PATH in $PROFILE"
		info "Run 'source $PROFILE' or open a new terminal, then run '$BINARY'"
	else
		info "Run '$BINARY' to get started"
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

	require_command uname
	require_command tar

	RECEIPT="$REQUESTY_HOME/version"
	OS="$(detect_os)"
	ARCH="$(detect_arch)"

	# A leftover marker file is how a user, or we, can tell that a previous run
	# died halfway through.
	MARKER_FILE="${TMPDIR:-/tmp}/requesty-cli-install.$(date +%Y%m%d%H%M%S).$$"
	: >"$MARKER_FILE"

	if [ "$VERSION" = "latest" ]; then
		VERSION="$(latest_version)"
	fi

	installed="$(installed_version)"
	if [ "$FORCE" = false ] && [ "$installed" = "$VERSION" ] && [ -x "$INSTALL_DIR/$BINARY" ]; then
		info "$BINARY $VERSION is already installed in $INSTALL_DIR"
		configure_path
		rm -f "$MARKER_FILE"
		exit 0
	fi

	if [ -n "$installed" ]; then
		info "Upgrading $BINARY from $installed to $VERSION"
	else
		info "Installing $BINARY $VERSION"
	fi

	install_release
	printf '%s\n' "$VERSION" >"$RECEIPT"

	configure_path
	rm -f "$MARKER_FILE"
	summary
}

main "$@"

}
