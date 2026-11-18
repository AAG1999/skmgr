# Tasks

## 1. Install docs

- [x] 1.1 Rewrite the README install section so it shows only `curl -fsSL https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.sh | sh` for macOS or Linux and `irm https://raw.githubusercontent.com/AAG1999/skmgr/main/scripts/install.ps1 | iex` for Windows PowerShell; verify the section does not mention Homebrew, `go install`, apt, rpm, apk, or a hand-downloaded zip.
- [x] 1.2 Rewrite `scripts/agent-install-prompt.md` so macOS, Linux, and WSL use `install.sh` and Windows PowerShell uses `install.ps1`; verify the file does not mention Homebrew, `go install`, or a package manager.
- [x] 1.3 Check `docs/how-it-works.md` and the rest of `docs/` for install instructions, and leave or edit them so they name only those two commands; verify a search of the docs for `brew install` and `go install github.com/AAG1999/skmgr` finds nothing.
- [x] 1.4 Record in `CHANGELOG.md` that the Homebrew cask and the deb, rpm, and apk packages are no longer published, and that a Homebrew user switches to `install.sh`; verify the unreleased section says that.

## 2. Release publishers

- [x] 2.1 Remove `homebrew_casks` and `nfpms` from `.goreleaser.yml`, and keep the archive builds and `scripts/install.sh` / `scripts/install.ps1` extra files; verify `goreleaser release --snapshot --clean --skip=publish` produces the platform tar.gz and zip archives and does not produce a cask, deb, rpm, or apk.

## Workflow follow-up

- Archive this change after review, then verify the archived result.
