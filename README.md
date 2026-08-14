# skmgr

**The framework-agnostic skill manager for AI agents**

[![CI](https://github.com/AAG1999/skmgr/actions/workflows/ci.yml/badge.svg)](https://github.com/AAG1999/skmgr/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/AAG1999/skmgr)](https://github.com/AAG1999/skmgr/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/AAG1999/skmgr)](https://go.dev/)
[![License](https://img.shields.io/github/license/AAG1999/skmgr)](https://github.com/AAG1999/skmgr/blob/main/LICENSE)

Declare AI agent skills and rules in a `skmgr.yml` manifest, lock them to git commits, and share them across your team. `skmgr` installs into a canonical `.agents/` store and symlinks into native agent directories — Cursor, Gemini, Claude Code, Copilot, and more.

## Highlights

- **Declarative manifest** — define skills once in `skmgr.yml`; names are derived for you
- **Lockfile reproducibility** — pin to git SHAs with `skmgr.lock`; `--frozen` for CI, `--offline` for air-gapped runs
- **Any git remote** — GitHub, GitLab, Bitbucket, or local `file://` paths
- **Semver without a registry** — follow release tags with ranges like `^1.2.0`
- **Agent-agnostic** — one install, many agents, via relative symlinks (junctions/copies on Windows)
- **Safe by default** — crash-safe writes, tamper detection (`skmgr verify`), orphan cleanup, `--dry-run` on mutating commands

## Install

```bash
# macOS
brew install AAG1999/tap/skmgr

# curl | sh (Linux / macOS)
curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh

# Go 1.22+
go install github.com/AAG1999/skmgr@latest
```

```powershell
# PowerShell (Windows)
irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex
```

**Ask an AI agent to install it** (copy-paste):

```text
Install skmgr (https://github.com/AAG1999/skmgr) on this machine.

Detect OS/arch yourself (Linux, macOS, Windows, or WSL). If `skmgr --version` already works, report it and stop.

Install with the first method that works:
- macOS: `brew install AAG1999/tap/skmgr`, else `curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh`
- Linux: `install.sh` one-liner
- Windows PowerShell: `irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex` (refresh PATH afterward)
- WSL: treat as Linux, not Windows
- Fallback: `go install github.com/AAG1999/skmgr@latest` (Go 1.22+)

Verify with `skmgr --version`. Warn if `git` is missing. Do not invent URLs. Show the commands you ran.
```

Full agent prompt: [`scripts/agent-install-prompt.md`](scripts/agent-install-prompt.md).


Pre-built binaries: [Releases](https://github.com/AAG1999/skmgr/releases).

## Quickstart

```bash
skmgr init
skmgr add https://github.com/anthropics/skills.git --path skills/frontend-design
skmgr install
skmgr list
```

```text
NAME              TYPE    SCOPE     REF    STATUS      TARGETS
frontend-design   skill   project   main   installed   cursor, gemini
```

## Manifest

```yaml
version: "1"
name: my-agent-workspace

targets:
  - cursor
  - gemini

skills:
  - source: https://github.com/anthropics/skills.git
    path: skills/skill-creator
    ref: main

  - source: https://github.com/anthropics/skills.git
    path: skills/pdf-tools
    ref: ^1.2.0

  - source: https://github.com/anthropics/skills.git
    path: "skills/*"
    ref: main
```

Full field reference, versioning, and lockfile details: [docs/reference/manifest.md](docs/reference/manifest.md).

## Commands

| Command | What it does |
|---------|--------------|
| `skmgr init` | Create `skmgr.yml` and `.agents/` |
| `skmgr add` / `remove` | Add or remove a dependency |
| `skmgr install` | Install from the manifest (`--frozen`, `--offline`) |
| `skmgr update` / `outdated` | Bump refs; see available updates |
| `skmgr list` / `tree` / `verify` | Inspect status and integrity |
| `skmgr doctor` | Diagnose the local environment |
| `skmgr clean` / `cache` | Clean installs and prune the cache |

All mutating commands support `--dry-run`. Full reference: [docs/reference/cli.md](docs/reference/cli.md).

## How it works

Skills are cached under `~/.skmgr/cache/`, copied into `.agents/`, then symlinked into agent directories (e.g. `.cursor/skills/`). Rules can be merged into single-file targets like `CLAUDE.md` using managed delimiters.

```bash
# CI
skmgr install --frozen
skmgr verify
```

Details: [docs/how-it-works.md](docs/how-it-works.md) · agent matrix: [docs/reference/agents.md](docs/reference/agents.md)

## Docs

| Document | Covers |
|----------|--------|
| [docs/](docs/README.md) | Full documentation index |
| [docs/how-it-works.md](docs/how-it-works.md) | Installing the CLI, symlinks, cache, Windows, CI |
| [docs/architecture.md](docs/architecture.md) | System design |
| [docs/flow.md](docs/flow.md) | Install / update / verify flows |
| [docs/reference/manifest.md](docs/reference/manifest.md) | Manifest, versioning, lockfile |
| [docs/reference/cli.md](docs/reference/cli.md) | Commands, flags, exit codes, config |
| [docs/reference/agents.md](docs/reference/agents.md) | Skills vs rules, agent paths |
| [docs/reference/](docs/reference/config.md) | Config, env vars, exit codes, private git |

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Short version:

```bash
gofmt -l . && go vet ./... && go test ./... && golangci-lint run
```

## License

Copyright 2026 AAG1999. Licensed under the [Apache License 2.0](LICENSE).
