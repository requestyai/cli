# Requesty CLI

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/requestyai/cli/blob/main/LICENSE)

> Point Claude Code, Codex, OpenCode, Pi and Hermes at [Requesty](https://requesty.ai) from one terminal app.

`requesty` finds the AI coding harnesses installed on your machine and rewrites their own
configuration so every request goes through the Requesty gateway. You get 300+ models behind a
single API key, one bill, and full logs and analytics for every tool you code with. It also shows
what you are spending, right in the terminal. There is no proxy to keep running and no wrapper
command to remember: your harnesses keep starting the way they always did.

Every file is backed up before it is touched, and existing settings are merged rather than
replaced by default.

**Contents:**

- [Quick start](#quick-start)
- [First run](#first-run)
- [Supported harnesses](#supported-harnesses)
- [Usage](#usage)
- [Merge or overwrite](#merge-or-overwrite)
- [Backups and how to revert](#backups-and-how-to-revert)
- [Keys](#keys)
- [Configuration](#configuration)
- [Advanced installation options](#advanced-installation-options)
- [Upgrade](#upgrade)
- [Uninstall](#uninstall)
- [Download a release manually](#download-a-release-manually)
- [Build from source](#build-from-source)
- [Troubleshooting](#troubleshooting)

## Quick start

```sh
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh
```

The installer adds `~/.requesty/bin` to your `PATH` and tells you which shell file it changed. Open
a new terminal, or run the `source` command it prints, then run `requesty`.

## First run

**Paste an API key**

On first run the CLI asks for a key. Create one on the
[API keys page](https://app.requesty.ai/api-keys). The key is checked against the gateway before
it is saved, so a typo is rejected here instead of failing later. It is stored in
`~/.requesty/config.json` and reused on every later run, so this happens once.

```text
╭──────────────────────────────────────────────────────────────────╮
│ Welcome to Requesty                                              │
│ One gateway for every model, in every tool you use, or app you   │
│ build.                                                           │
│                                                                  │
│ Paste an API key from https://app.requesty.ai/api-keys to begin. │
│                                                                  │
│ ❯ rqsty-...                                                      │
│                                                                  │
│ enter continue · ctrl+c quit                                 dev │
╰──────────────────────────────────────────────────────────────────╯
```

**Pick a harness**

Harnesses found on your `PATH` are listed first. The ones that are not installed stay dimmed and
cannot be configured. The footer always shows the keys available on the current screen.

```text
  harnesses on this machine                    0 of 5 routing through requesty

        HARNESS             CONFIG                                  STATUS
  ────────────────────────────────────────────────────────────────────────────
  ❯ [ ] Claude Code         /home/you/.claude/settings.json         inactive
    [ ] Codex               /home/you/.codex/config.toml            inactive
    [ ] OpenCode            /home/you/.config/opencode/opencode.jsoninactive
    [ ] Pi                  /home/you/.pi/agent/models.json         inactive
    [ ] Hermes              /home/you/.hermes/config.yaml           inactive

  ╭──────────────────────────────────────────────────────────────────────────╮
  │ Claude Code                              /home/you/.claude/settings.json │
  │ takes a backup of settings.json                                          │
  │ writes a settings.json to route through Requesty                         │
  ╰──────────────────────────────────────────────────────────────────────────╯

  ↑/↓ move · space configure · r refresh · q/esc quit
```

**Choose a model, then choose how to write the config**

The model list is fetched from your account with your key, so it reflects the models and policies
you actually have access to. Then choose **merge** (recommended) or **overwrite**. The CLI writes
the files, the row flips to `[✓] active`, and the header count goes up.

**Restart the harness**

Harnesses read their configuration at startup. Restart `claude`, `codex`, `opencode`, `pi` or
`hermes` and it is talking to Requesty.

## Supported harnesses

| Harness | Detected by | Files written | What Requesty sets |
| --- | --- | --- | --- |
| [Claude Code](https://docs.requesty.ai/integrations/claude-code) | `claude` on `PATH` | `~/.claude/settings.json` | `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_MODEL` in the `env` block |
| [Codex](https://docs.requesty.ai/integrations/openai-codex) | `codex` on `PATH` | `~/.codex/config.toml`, `~/.codex/auth.json` | A `requesty` model provider on `.../v1`, the selected model, and API key auth |
| [OpenCode](https://docs.requesty.ai/integrations/opencode) | `opencode` on `PATH` | `~/.config/opencode/opencode.json` | A `requesty` provider on `.../v1` plus the model as `requesty/<model>` |
| [Pi](https://docs.requesty.ai/integrations/pi) | `pi` on `PATH` | `~/.pi/agent/models.json` | A `requesty` provider using the native Anthropic Messages API |
| [Hermes](https://docs.requesty.ai/integrations/hermes) | `hermes` on `PATH` | `~/.hermes/config.yaml` | A `requesty` entry in `custom_providers` and `model.default` |

Pi and Hermes are configured against the native Anthropic Messages format, which is what lets
Requesty apply [automatic prompt caching](https://docs.requesty.ai/features/auto-caching) to
those harnesses.

Where a harness supports custom headers, the CLI also sets an `X-Title` header naming the tool, so
the [Requesty dashboard](https://app.requesty.ai/analytics) can break spend down per harness.

## Usage

Above the harness list, the CLI shows spend, requests and tokens for the last 30 days for the key
you onboarded with, and refreshes them on demand. Full breakdowns by model, user and tool live in
the [Requesty dashboard](https://app.requesty.ai/analytics).

## Merge or overwrite

The last step of the wizard asks how to write the configuration.

- **Merge existing config files (recommended).** Your settings are kept and only the Requesty
  values are added or updated. Comments and formatting of TOML and YAML files are not preserved,
  because the files are re-encoded.
- **Overwrite config files.** The file is replaced with only what Requesty needs. Use this when a
  config has drifted and you want a clean, known-good state.

Either way, a backup is taken first.

## Backups and how to revert

Before the first write, each file is copied to `<file>.requesty.bak`, keeping its original
permissions. The backup is only created once per file, so your true pre-Requesty state is never
overwritten by a later run.

To go back to your original setup:

```sh
mv ~/.claude/settings.json.requesty.bak ~/.claude/settings.json
```

## Keys

Your Requesty API key is written into `~/.requesty/config.json` and into each harness config the
CLI configures, because that is how those harnesses authenticate. All of these files are written
so that only your user can read them.

Treat those files as secrets and do not commit them. Keys can be rotated or revoked at any time
on the [API keys page](https://app.requesty.ai/api-keys).

## Configuration

`~/.requesty/config.json`:

```json
{
	"api_key": "rqsty-...",
	"router_base_url": "https://router.requesty.ai"
}
```

| Field | Meaning |
| --- | --- |
| `api_key` | The key written into harness configs and used for API calls |
| `router_base_url` | Inference endpoint the harnesses are pointed at |

Change `router_base_url` to route through a different region or a self-hosted deployment, for
example `https://eu.router.requesty.ai`. Delete the file to start over from onboarding.

## Advanced installation options

The installer downloads the latest release, verifies its checksum, installs the binary into
`~/.requesty/bin`, and adds that directory to your `PATH` in `.zshrc`, `.bashrc` or `config.fish`.
Open a new shell, or source your shell config, and `requesty` is on your `PATH`.

It accepts a few options, for example to pin a version or install somewhere else:

```sh
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh -s -- --version v1.2.3
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh -s -- --install-dir ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh -s -- --no-modify-path
```

The same settings are available as environment variables: `REQUESTY_VERSION`,
`REQUESTY_INSTALL_DIR` and `REQUESTY_HOME`. Run `install.sh --help` for the full list.

## Upgrade

Run the install command again. The binary is replaced in place and everything in `~/.requesty`,
including `config.json`, is preserved:

```sh
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh
```

## Uninstall

Restore any harness config you want back from its `.requesty.bak` backup, then:

```sh
rm -rf ~/.requesty
```

Remove the block marked `# >>> requesty cli installer >>>` from your shell configuration file to
take the install directory back off your `PATH`.

## Download a release manually

| Platform | Command |
| --- | --- |
| macOS (Apple Silicon) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_darwin_arm64.tar.gz` |
| macOS (Intel) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_darwin_amd64.tar.gz` |
| Linux (ARM64) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_linux_arm64.tar.gz` |
| Linux (x86_64) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_linux_amd64.tar.gz` |
| Windows (ARM64) | `curl.exe -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_windows_arm64.zip` |
| Windows (x86_64) | `curl.exe -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_windows_amd64.zip` |

Unpack the archive and put the `requesty` binary anywhere on your `PATH`.

## Build from source

The Go version in [`go.mod`](go.mod) is what CI builds with.

```sh
go run .            # run the CLI
go build -o requesty .
go test ./...
gofmt -l .          # CI fails if this prints anything
```

## Troubleshooting

**A harness is listed but cannot be configured.** Its executable was not found on your `PATH`.
Install the harness, then refresh the list.

**`That key was not recognised`.** The gateway does not accept the key you pasted. Copy it again
from the [API keys page](https://app.requesty.ai/api-keys).

**`Could not load usage`.** The usage panel calls the management API with your key. A `401` means
the key was revoked or has expired: create a new one on the
[API keys page](https://app.requesty.ai/api-keys) and delete `~/.requesty/config.json` to
re-onboard. Routing itself is unaffected by this panel.

**The harness still uses its old provider.** Restart it. Configuration is read at startup.

## Learn more

- [Requesty docs](https://docs.requesty.ai)
- [Model library](https://app.requesty.ai/model-list)
- [Discord](https://discord.com/invite/Td3rwAHgt4)
