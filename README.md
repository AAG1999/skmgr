# skmgr

**The framework-agnostic skill manager for AI agents**

[![CI](https://github.com/AAG1999/skmgr/actions/workflows/ci.yml/badge.svg)](https://github.com/AAG1999/skmgr/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/AAG1999/skmgr)](https://github.com/AAG1999/skmgr/releases)
[![Go Version](https://img.shields.io/github/go-mod/go-version/AAG1999/skmgr)](https://go.dev/)
[![License](https://img.shields.io/github/license/AAG1999/skmgr)](https://github.com/AAG1999/skmgr/blob/main/LICENSE)

skmgr declares AI agent skills and rules in `skmgr.yml`, locks them to git commits, and installs them for Cursor, Gemini, Claude Code, and Copilot.

## See it in action

```text
You: skmgr init
AI:  Which agents should skmgr manage?
You: cursor
AI:  Getting started
       /skmgr-find-skill
       /skmgr-install-skill
       /skmgr-update-skill
       /skmgr-remove-skill
       /skmgr-create-skill
     Restart your IDE to refresh skills.
You: /skmgr-find-skill a skill that fills PDF forms
AI:  Here are repositories that match. Which URL should I inspect?
```

## Install

On macOS or Linux, in your terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh
```

On Windows, in PowerShell:

```powershell
irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex
```

An agent can do this for you. The prompt is [`scripts/agent-install-prompt.md`](scripts/agent-install-prompt.md).

## Docs

→ **[Documentation](docs/README.md)**: the full index
→ **[How it works](docs/how-it-works.md)**: install, symlinks, cache, Windows, CI
→ **[Manifest](docs/reference/manifest.md)**: fields, versioning, lockfile
→ **[CLI](docs/reference/cli.md)**: commands, flags, exit codes
→ **[Workflow skills](docs/reference/workflow-skills.md)**: the chat skills and slash commands

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). The short check before a change is `gofmt -l . && go vet ./... && go test ./... && golangci-lint run`.

## License

Copyright 2026 AAG1999. Licensed under the [Apache License 2.0](LICENSE).
