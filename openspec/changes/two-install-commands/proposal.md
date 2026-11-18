# Proposal

## Why

The README offers Homebrew, a shell one-liner, `go install`, a PowerShell one-liner, an agent prompt with fallbacks, and a release download. People and agents pick different paths and get different installs. OpenSpec's quick start is two commands. skmgr should be the same.

## What Changes

- The only documented ways to install the `skmgr` binary are `scripts/install.sh` on macOS or Linux (including WSL) and `scripts/install.ps1` on Windows PowerShell.
- The agent install prompt uses those two commands and nothing else.
- **BREAKING**: stop publishing the Homebrew cask. `brew install AAG1999/tap/skmgr` is no longer a supported install.
- **BREAKING**: stop publishing the deb, rpm, and apk packages. They are further install paths.
- Release archives stay. Both scripts download `skmgr_<OS>_<arch>.tar.gz` or the Windows zip from the GitHub release.
- `skmgr self-update` stays. It updates a binary that is already installed.
- `go install` and a hand-downloaded zip leave the install docs. The Go module is unchanged, so `go install` still builds, but it is not an offered method.

## Capabilities

### New Capabilities

- `binary-install`: The two commands that install the skmgr binary, and the install methods the project no longer offers.

### Modified Capabilities

## Impact

- Docs: `README.md`, `docs/how-it-works.md`, `scripts/agent-install-prompt.md`, `CHANGELOG.md`, and any other page that lists Homebrew, `go install`, a package manager, or a manual zip as how to install.
- Release: `.goreleaser.yml` drops `homebrew_casks` and `nfpms`. The snapshot build no longer emits a cask, deb, rpm, or apk.
- The two installer scripts, their CI workflow, and the release zip and tar.gz archives stay.
