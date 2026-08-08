# Requesty CLI

## Install

On macOS and Linux:

```sh
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh
```

The installer downloads the latest release, verifies its checksum, installs the
binary into `~/.requesty/bin` and adds that directory to your `PATH` in
`.zshrc`, `.bashrc` or `config.fish`.

## Upgrade

Run the same command again. Everything in `~/.requesty`, including
`config.json`, is preserved:

```sh
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh
```

The installer accepts a few options, for example to pin a version or to install
somewhere else:

```sh
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh -s -- --version v1.2.3
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh -s -- --install-dir ~/.local/bin
curl -fsSL https://raw.githubusercontent.com/requestyai/cli/main/install.sh | sh -s -- --no-modify-path
```

Run `install.sh --help` for the full list.

## Run from source

```sh
go run .
```

## Download the latest release manually

Download the binary for your platform:

| Platform | Command |
| --- | --- |
| macOS (Apple Silicon) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_darwin_arm64.tar.gz` |
| macOS (Intel) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_darwin_amd64.tar.gz` |
| Linux (ARM64) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_linux_arm64.tar.gz` |
| Linux (x86_64) | `curl -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_linux_amd64.tar.gz` |
| Windows (ARM64) | `curl.exe -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_windows_arm64.zip` |
| Windows (x86_64) | `curl.exe -fLO https://github.com/requestyai/cli/releases/latest/download/requesty_windows_amd64.zip` |