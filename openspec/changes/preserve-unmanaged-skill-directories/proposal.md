# Proposal

## Why

`skmgr add`, `install`, `update`, `sync`, and `remove` all finish in `Engine.Sync`, which deletes every directory under the project's `.agents/skills/` and `.agents/rules/` whose name is not in the set just resolved. Skills and rules that another tool wrote there — OpenSpec's seven skills in Pulse-SkillHub, for example — are removed even though they were never in `skmgr.yml` or `skmgr.lock`. The global tree already refuses to do this: `cleanGlobalOrphans` adopts directories `~/.skmgr/global.lock` does not claim. The project tree has no such check.

## What Changes

- Project-scope cleanup during sync deletes a canonical directory only when its name is in the previous lockfile and the manifest does not resolve that name with project scope. A name that was never locked is not a candidate. Lock entries do not store scope, so a name that moves from project scope to global scope still loses its project directory.
- A directory skmgr did not record is left in place, including on the first sync when no lockfile exists yet. Non-directory files in those trees, such as `.openspec-target`, stay as they do today.
- A skill or rule that skmgr did install, and that a later sync drops from the manifest, is still removed from `.agents/skills/` or `.agents/rules/`.
- Reserved workflow directories (`skmgr-find-skill`, `skmgr-install-skill`, `skmgr-update-skill`, `skmgr-remove-skill`, `skmgr-create-skill`) are never removed by this pass.
- Global cleanup is unchanged: `~/.agents/` is still pruned from the ledger union after unclaimed directories are adopted.
- Add a regression test that an unmanaged directory under `.agents/skills/` survives `add` and `install`, and that a previously locked name that leaves the manifest does not.

Out of scope: `skmgr clean`, which is an explicit wipe of the canonical store (it already spares reserved workflow names). Recording scope on lock entries. Adopting unmanaged project directories into `skmgr.lock` — that file is the list of what skmgr installed, and writing an unmanaged name into it would schedule the directory for deletion on the next sync. The global ledger can adopt because it is a keep-set; the project lockfile is not.

## Capabilities

### New Capabilities

- `project-orphan-cleanup`: Which directories under a project's `.agents/skills/` and `.agents/rules/` a sync may delete, and which directories it must leave alone.

### Modified Capabilities

- None. `lockfile` covers round-trip of entry strings, and `binary-install` covers how the `skmgr` binary is installed. Neither states what sync may delete.

## Impact

- `Engine.Sync` in `internal/engine/engine.go` (the project `CleanOrphans` call around the existing lock) and `Installer.CleanOrphans` / `CleanOrphansByName` in `internal/engine/installer.go`. Global `cleanGlobalOrphans` stays.
- `TestCleanOrphans_RemovesUnlisted` currently requires deleting a directory that was never locked. That expectation is the bug. Command tests in `cmd/install_test.go` (`TestInstall_RemovesOrphanWhenSkillDropped`) must keep passing.
- `docs/flow.md` says project orphans are decided from this project's manifest alone. That sentence has to match the lockfile rule. `docs/architecture.md` describes the global ledger only; no change required there unless a project-cleanup sentence is added for symmetry.
- No lockfile format change, no new dependency, no CLI flag. `add`, `install`, `update`, `sync`, and `remove` pick up the behavior because they call `Sync`.
