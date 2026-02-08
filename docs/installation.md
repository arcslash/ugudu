---
layout: default
title: Installation
---

# Installation

Ugudu ships as a single binary that includes the CLI, daemon, web UI, and MCP server.

## Quick Install

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/arcslash/ugudu/main/install.sh | bash
```

This downloads the latest release and installs to `/usr/local/bin`.

### Windows (PowerShell)

```powershell
irm https://raw.githubusercontent.com/arcslash/ugudu/main/install.ps1 | iex
```

Installs to `%LOCALAPPDATA%\Programs\Ugudu` and adds to PATH.

## Package Managers

### Homebrew (macOS/Linux)

```bash
brew tap arcslash/ugudu https://github.com/arcslash/ugudu
brew install ugudu
```

### Chocolatey (Windows)

```powershell
choco install ugudu
```

### npm

```bash
npm install -g @arcslash/ugudu
```

### Go

```bash
go install github.com/arcslash/ugudu/cmd/ugudu@latest
```

## Manual Download

Download binaries directly from [GitHub Releases](https://github.com/arcslash/ugudu/releases):

| Platform | Architecture | File |
|----------|--------------|------|
| macOS | Apple Silicon | `ugudu_VERSION_darwin_arm64.tar.gz` |
| macOS | Intel | `ugudu_VERSION_darwin_amd64.tar.gz` |
| Linux | x64 | `ugudu_VERSION_linux_amd64.tar.gz` |
| Linux | ARM64 | `ugudu_VERSION_linux_arm64.tar.gz` |
| Windows | x64 | `ugudu_VERSION_windows_amd64.zip` |

Extract and move to a directory in your PATH:

```bash
# macOS/Linux
tar xzf ugudu_*.tar.gz
sudo mv ugudu /usr/local/bin/

# Windows - extract zip and add to PATH
```

## Build from Source

Requirements: Go 1.22+

```bash
git clone https://github.com/arcslash/ugudu.git
cd ugudu
make build
sudo make install
```

## Verify Installation

```bash
ugudu version
```

## Next Steps

1. [Configure API keys](configuration)
2. [Start the daemon](quickstart)
3. [Create your first team](quickstart#create-a-team)
