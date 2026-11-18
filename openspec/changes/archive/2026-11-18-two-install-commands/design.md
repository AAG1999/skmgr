# Design

## Context

See proposal.md for why the install surface shrinks. Today `README.md` lists Homebrew, `scripts/install.sh`, `go install`, and `scripts/install.ps1`, plus an agent prompt with those fallbacks and a link to download a release by hand. `.goreleaser.yml` publishes a Homebrew cask and deb, rpm, and apk packages, and it also publishes the tar.gz and zip archives the two scripts download. `docs/how-it-works.md` already shows only the two scripts.

## Goals / Non-Goals

**Goals:**

- The README install section and the agent prompt match the two commands in `binary-install`.
- A release snapshot contains the platform archives and does not contain a cask, deb, rpm, or apk.

**Non-Goals:**

- Changing what `install.sh` or `install.ps1` download, or where they install.
- Changing `skmgr self-update`.
- Removing the public Go module. `go install` still builds; the docs stop offering it.
- Deleting an already published cask from the `homebrew-tap` repository. That repo is not this one.

## Decisions

### D1. Keep the two scripts and the archives they fetch

The scripts already implement the two commands. They resolve the latest GitHub release and download `skmgr_<OS>_<arch>.tar.gz` or the Windows zip. Removing those archives would break both commands, so `release.extra_files` and the archive builds stay.

Alternative: host the scripts on a short domain the way OpenSpec uses `openspec.dev/cli`. Rejected. The GitHub raw URLs already work, and a new host is another install path to run.

### D2. Drop `homebrew_casks` and `nfpms` from GoReleaser

Those blocks are what publishes the extra install methods. Deleting them is the release change. The CI snapshot job (`goreleaser release --snapshot`) then proves the cask and the Linux packages are gone and the archives remain.

Alternative: leave the publishers and only edit the README. Rejected. `brew install` and `apt` would still be ways to install.

### D3. Rewrite the install docs in place

The README install section becomes the two commands, in the same order as the OpenSpec quick start: macOS or Linux first, Windows PowerShell second. `scripts/agent-install-prompt.md` picks `install.sh` for macOS, Linux, and WSL, and `install.ps1` for Windows PowerShell. `docs/how-it-works.md` already matches; the task checks it and leaves it when the wording is already the two commands. `CHANGELOG.md` records that the cask and the Linux packages are no longer published, and tells a Homebrew user to install with `install.sh`.

## Risks / Trade-offs

- [Someone on an old cask keeps that binary] → The changelog says the cask will not receive new releases and points them at `install.sh`. Removing the cask file from `homebrew-tap` is a separate edit in that repository.
- [A Linux user who installed the deb has no package upgrade] → The changelog says to switch to `install.sh`. `skmgr self-update` still updates that binary once it is the one on `PATH`.
- [`go install` still works from the module] → Accepted. It is not documented as an install method.

## Migration Plan

1. Edit the docs so the two commands are the only ones shown.
2. Remove `homebrew_casks` and `nfpms` from `.goreleaser.yml`.
3. Run a GoReleaser snapshot and confirm the archives exist and the cask, deb, rpm, and apk do not.
4. On the next tagged release, the tap stops receiving a new cask. Tell current Homebrew users, in the changelog, to run `install.sh`.

Rollback is putting the two GoReleaser blocks back and restoring the README section.

## Open Questions
