# Proposal

## Why

`README.md` has become a second copy of the docs. Highlights, a full manifest, a command table, a how-it-works note, and a docs table all repeat pages that already exist under `docs/`, and the docs table still says there are four chat skills. OpenSpec's README is the shape to follow: what the tool is, a short conversation that shows it, how to install, then links.

## What Changes

- Rewrite `README.md` so a new reader can install skmgr, run `skmgr init`, and see the chat skills without reading the rest of the repository.
- Keep the two install commands (`scripts/install.sh` and `scripts/install.ps1`) and the link to `scripts/agent-install-prompt.md`.
- Replace the quickstart prose with a short conversation: init, restart the IDE, then find, install, update, remove, and create.
- Remove the highlights list, the manifest sample, the command table, the how-it-works blurb, and the docs table from the README. Those stay in the docs pages they already link to.
- End with a short list of doc links, a one-paragraph contributing note, and the license.
- No CLI, lockfile, workflow, or install-script behavior changes.

## Capabilities

### New Capabilities

- None. This change only rewrites documentation.

### Modified Capabilities

- None. The `lockfile` spec is unchanged.

## Impact

- `README.md` is the only file rewritten.
- `docs/README.md` and the pages it indexes stay as they are. The README links to them instead of restating them.
- Badges, the Apache-2.0 license line, and the pointer to `CONTRIBUTING.md` stay.
