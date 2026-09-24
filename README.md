# Requesty CLI

[![License: Apache 2.0](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/requestyai/cli/blob/main/LICENSE)

> Point Claude Code, Codex, OpenCode, Pi, Hermes and DeepSeek Harness at [Requesty](https://requesty.ai) from one terminal app.

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
- [Launching a harness](#launching-a-harness)
- [Profiles](#profiles)
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

**Sign in with your browser**

The first time you run `requesty` or a harness command such as `requesty claude` on a machine without a profile,
the CLI opens with a welcome page that says what is about to happen. Press enter and it signs you in
to Requesty in your browser; once you approve, it creates an API key in your account, named after
this machine (`Requesty CLI (my-laptop)`), and saves it as a [profile](#profiles) in
`~/.requesty/config.json`. The app then carries on to its dashboard, or the harness you asked for
starts. Every later run reuses that profile. The key shows up on the
[API keys page](https://app.requesty.ai/api-keys) like any other, and can be revoked there.

```text
╭──────────────────────────────────────────────────────────────────╮
│ Welcome to Requesty                                              │
│ One gateway for every model, in every tool you use, or app you   │
│ build.                                                           │
│                                                                  │
│ Sign in with your browser to get started. This creates an API    │
│ key named "Requesty CLI (my-laptop)" in your Requesty account    │
│ and saves it to /home/you/.requesty/config.json.                 │
│ Claude Code starts as soon as the key is saved.                  │
│                                                                  │
│ Working over SSH, or already have a key? Quit and run `requesty  │
│ login --api-key <key>` instead.                                  │
│                                                                  │
│ enter sign in · q/esc quit                                   dev │
╰──────────────────────────────────────────────────────────────────╯
```

While the browser is open a dialog shows the sign-in address, for when the browser did not open on
its own; `esc` cancels. The key is created in your group. If you belong to several groups a dialog
asks which one; if you belong to none it is a personal key. Organizations that require keys to live
in a group will say so, in which case ask an admin to add you to one. The profile is named
`default`; pass `--profile <name>` to choose another name.

**`requesty login`**

`requesty login` runs the same sign-in from the command line. Use it to set up a machine before
launching anything, to pick a group without being asked (`--group Engineering`), to name the
profile yourself (`--profile eu`), to point it at another router (`--router-url`), or to replace a
key that was revoked or has expired. Existing profiles are left alone unless you pass `--force`.

**Over SSH, or with a key you already have**

The browser hands the sign-in back to the CLI on `127.0.0.1`, which does not work over SSH. Create
a key on the [API keys page](https://app.requesty.ai/api-keys) and run
`requesty login --api-key <key>` instead. The key is checked against the gateway before it is
saved, so a typo is rejected here instead of failing later. It is saved as the profile `default`
unless you pass `--profile`.

## Launching a harness

`requesty claude`, `requesty codex`, `requesty opencode`, `requesty pi` and `requesty hermes`
start the harness with its traffic routed through Requesty for that run only; the harness's own
configuration is not changed. The current profile is used unless you select another.

The first time you launch a harness from a profile, the CLI settles which model it should use and
remembers the answer in the profile. When the key can route to the harness's recommended default
(`claude-sonnet-4-6` for Claude Code), that is used and printed, without asking. Otherwise a
picker asks: the Policies tab lists Requesty's managed policies, which name a model once and
route it across providers, and `tab` switches to every model the key can route to. `--model`
overrides the remembered model for one run without changing it; `--choose-model` opens the picker
to change it. In a script or CI with no terminal to ask in, the default is still used when it can
be; otherwise pass `--model`, or pick once interactively first.

Claude Code hands background work (session titles, summaries, subagents marked `haiku`) to a
smaller model, which it would otherwise ask for by an Anthropic model id the gateway does not
know. The CLI settles this the same way: `claude-haiku-4-5` when the key can route to it, else the
main model itself, so an access list without Haiku still works. `--choose-fast-model` opens the
picker to change it and `--fast-model` overrides it for one run.

```sh
requesty claude                                    # current profile, remembered model (settled the first time)
requesty claude --profile eu                       # a named profile, with its own remembered model
requesty claude --model anthropic/claude-opus-4-1  # this model, just for this run
requesty claude --choose-model                     # pick again and remember the new answer
requesty claude --choose-fast-model                # pick what background work runs on
requesty codex --reasoning-effort high -- --full-auto
requesty opencode --model gpt-5.5 -- run "explain this repo"
requesty pi --reasoning-effort medium
requesty hermes -- chat -q "hello"
```

Profile precedence is `--profile <name>`, then `REQUESTY_PROFILE`, then the current profile set
with `requesty profiles use`. For a harness command, put `--profile` after the harness name and
before the first argument that is passed through to the harness.

Each harness is pointed at Requesty in the way it allows without touching its files:

- **Claude Code** gets `ANTHROPIC_BASE_URL` and the key through the environment plus a
  `--settings` document for this run.
- **Codex** gets a `requesty` model provider through `-c` overrides.
- **OpenCode** knows Requesty as a built-in provider that switches on when `REQUESTY_API_KEY` is
  set; an `OPENCODE_CONFIG_CONTENT` document, merged into any you already export, pins it to this
  profile, and the model is passed as `-m requesty/<model>`. A Requesty key stored earlier with
  `opencode auth login` can take precedence over the profile's; the CLI warns when the two differ.
- **Pi** has no flag for a custom provider, so the CLI writes a small extension and a catalog of
  the models your key can route to under `~/.requesty/pi/` and starts Pi with `--extension`. The
  catalog is refreshed from your account on each launch and kept from last time when that fails.
  Other vendors' API key variables are dropped from Pi's environment so every model it offers
  goes through Requesty.
- **Hermes** is started as `--provider custom` with `CUSTOM_BASE_URL` pointing at the router.
  Hermes loads `~/.hermes/.env` over the environment, so the CLI warns when a value there would
  replace one of its own.

Reasoning effort (`--reasoning-effort minimal|low|medium|high|xhigh|max`) maps onto each harness's
own flag; levels a harness does not support are rejected before it starts. Flags you pass through
for the model, provider or effort take precedence over the CLI's.

## Profiles

A profile is one sign-in: an API key and the router it is sent to. Most people have one. You want
more when your organization gives you keys in
different groups with different model policies (an EU group and a US group, say), or when you use
Requesty for work and personally on the same machine. Keys with manage permissions for the
`api-keys`, `groups` and `access-lists` subcommands fit here too: save one with
`requesty login --api-key <key> --profile manage` and pass `--profile manage` to those commands.

```sh
requesty profiles list                                  # what is saved, current marked with *
requesty profiles use eu                                # make this profile current
requesty profiles remove personal                       # forget a profile (the key is not revoked)
```

`requesty login` creates the `default` profile unless you pass `--profile`. The first profile
becomes current. Every command accepts `--profile <name>`, and `REQUESTY_PROFILE` does the same
for a whole shell.

Harnesses configured from the dashboard stay pinned to the profile used during configuration.
Claude Code stores that profile's key in its settings. Codex fetches the key through
`requesty auth token --profile <name>`. Configure the harness again to move it to another profile.

**Pick a harness**

Harnesses found on your `PATH` are listed first. The ones that are not installed stay dimmed and
cannot be configured. The footer always shows the keys available on the current screen.

```text
  harnesses on this machine                    0 of 6 routing through requesty

        HARNESS             CONFIG                                  STATUS
  ────────────────────────────────────────────────────────────────────────────
  ❯ [ ] Claude Code         /home/you/.claude/settings.json         inactive
    [ ] Codex               /home/you/.codex/config.toml            inactive
    [ ] OpenCode            /home/you/.config/opencode/opencode.jsoninactive
    [ ] Pi                  /home/you/.pi/agent/models.json         inactive
    [ ] Hermes              /home/you/.hermes/config.yaml           inactive
    [ ] DeepSeek Harness    /home/you/.dsh/settings.yaml            inactive

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

Harnesses read their configuration at startup. Restart `claude`, `codex`, `opencode`, `pi`,
`hermes` or `dsh` and it is talking to Requesty.

## Supported harnesses

| Harness | Detected by | Files written | What Requesty sets |
| --- | --- | --- | --- |
| [Claude Code](https://docs.requesty.ai/integrations/claude-code) | `claude` on `PATH` | `~/.claude/settings.json` | `ANTHROPIC_BASE_URL`, `ANTHROPIC_AUTH_TOKEN` and `ANTHROPIC_MODEL` in the `env` block |
| [Codex](https://docs.requesty.ai/integrations/openai-codex) | `codex` on `PATH` | `~/.codex/config.toml` | A `requesty` model provider on `.../v1`, the selected model, and command-backed auth through `requesty auth token` |
| [OpenCode](https://docs.requesty.ai/integrations/opencode) | `opencode` on `PATH` | `~/.config/opencode/opencode.json` | A `requesty` provider on `.../v1` plus the model as `requesty/<model>` |
| [Pi](https://docs.requesty.ai/integrations/pi) | `pi` on `PATH` | `~/.pi/agent/models.json` | A `requesty` provider using the native Anthropic Messages API |
| [Hermes](https://docs.requesty.ai/integrations/hermes) | `hermes` on `PATH` | `~/.hermes/config.yaml` | A `requesty` entry in `custom_providers` and `model.default` |
| [DeepSeek Harness](https://docs.requesty.ai/integrations/deepseek-harness) | `dsh` on `PATH` | `$DSH_HOME/settings.yaml`, `$DSH_HOME/.credentials.yaml` (`~/.dsh` by default) | A `requesty` provider on `.../v1` under `llm-pi-ai`, the selected model as `agent-default-model`, and the key stored as `REQUESTY_API_KEY` |

Pi and Hermes are configured against the native Anthropic Messages format, which is what lets
Requesty apply [automatic prompt caching](https://docs.requesty.ai/features/auto-caching) to
those harnesses.

Where a harness supports custom headers, the CLI also sets an `X-Title` header naming the tool, so
the [Requesty dashboard](https://app.requesty.ai/analytics) can break spend down per harness.

DeepSeek Harness needs the models of a custom provider listed in its own settings, so the CLI
writes the model you selected. You can add more Requesty models later inside the harness, under
Settings, Models, Requesty, either with `Fetch available models` or with `Add model`. In merge mode
a later run of the CLI keeps those entries and only adds the model you selected if it is missing.

## Usage

Above the harness list, the CLI shows spend, requests and tokens for the last 30 days for the
profile it is running as, and refreshes them on demand. Full breakdowns by model, user and tool
live in the [Requesty dashboard](https://app.requesty.ai/analytics).

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

Your Requesty API keys are written into `~/.requesty/config.json`, one per profile. Codex
retrieves the current profile's key when needed through `requesty auth token`; other harnesses
may also store it in their own config because that is how they authenticate. All files
containing a key are written so that only your user can read them.

A key created by signing in can make completions and read its own usage, nothing more. The
management subcommands (`requesty api-keys`, `requesty groups`, `requesty access-lists`) need a
key with manage permissions from the [API keys page](https://app.requesty.ai/api-keys), saved as
its own profile with `requesty login --api-key <key> --profile <name>`. The short-lived token from
the browser sign-in is used once to create the key and is never stored.

Treat those files as secrets and do not commit them. Keys can be rotated or revoked at any time
on the [API keys page](https://app.requesty.ai/api-keys); run `requesty login` afterwards to get
a new one. Removing a profile with `requesty profiles remove` forgets the key locally only.

## Configuration

`~/.requesty/config.json`:

```json
{
	"current": "engineering",
	"profiles": {
		"engineering": {
			"api_key": "rqsty-..."
		},
		"eu": {
			"api_key": "rqsty-...",
			"router_base_url": "https://router.eu.requesty.ai"
		}
	}
}
```

- `current` names the profile used when none is selected explicitly. It is required whenever any
  profiles are saved.
- `profiles.<name>.api_key` authenticates API and harness requests.
- `profiles.<name>.router_base_url` is that profile's inference endpoint. When omitted, it defaults
  to `https://router.requesty.ai`.

Set a profile's `router_base_url` to use a different region or self-hosted deployment. The config
has no legacy single-key format; delete the file to start over from onboarding.

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

**`that API key was not recognised`.** The gateway does not accept the key you gave
`requesty login --api-key`. Copy it again from the [API keys page](https://app.requesty.ai/api-keys).

**The browser never comes back to the terminal.** The sign-in finishes by redirecting your browser
to `127.0.0.1` on the machine running the CLI, so it cannot complete over SSH or in a container.
Run `requesty login --api-key <key>` with a key from the
[API keys page](https://app.requesty.ai/api-keys) instead, or forward the port shown in the
sign-in URL.

**`no Requesty profile configured; run requesty login`.** A command that cannot onboard
automatically ran without a saved profile. Run `requesty login` or
`requesty login --api-key <key>`.

**`group_id is required for organizations in group budget mode`.** Your organization tracks spend
per group, and you are not in one yet. Ask an organization admin to add you to a group, then sign
in again.

**A harness gets a `401`.** The key was revoked or has expired. Replace that profile
with `requesty login --profile <name> --force`. With OpenCode, also check for a Requesty key stored
by `opencode auth login`, which wins over the profile's; `opencode auth logout` removes it. With
Hermes, check `~/.hermes/.env` for a `REQUESTY_API_KEY` or `CUSTOM_BASE_URL` entry, which Hermes
loads over the environment.

**`Requesty needs pi 0.84.0 or newer`.** `requesty pi` registers Requesty through Pi's provider
extension API, which arrived in Pi 0.84.0. Upgrade Pi and try again.

**`Could not load usage`.** The usage panel asks the management API about your own key, which any
key may do. A `401` means the key was revoked or has expired: run `requesty login` to get a new
one. Routing itself is unaffected by this panel.

**The harness still uses its old provider.** Restart it. Configuration is read at startup.

## Learn more

- [Requesty docs](https://docs.requesty.ai)
- [Model library](https://app.requesty.ai/model-list)
- [Discord](https://discord.com/invite/Td3rwAHgt4)
