# Design

## Context

See proposal.md for why unmanaged directories disappear. The specs in `specs/project-orphan-cleanup/spec.md` are the behavior contract.

`Engine.Sync` (`internal/engine/engine.go`) already receives `existingLock`. After resolving, `removeDroppedEntries` unlinks names that were in that lock and are absent from the resolved set. Later, `Installer.CleanOrphans(resolvedSkills, ScopeProject)` builds a keep-set of names whose `EffectiveScope()` is project and `CleanOrphansByName` deletes every other directory under `.agents/skills/` and `.agents/rules/`, skipping only `types.ReservedWorkflowNames()`. `LockEntry` has no scope and no type. `CleanOrphans` is called from `Sync` and from `TestCleanOrphans_RemovesUnlisted`; nothing else uses it. Global pruning is separate: `cleanGlobalOrphans` adopts unclaimed names into `~/.skmgr/global.lock`, then `CleanOrphansByName(ledger.AllNames(), ScopeGlobal)`.

`docs/flow.md` states that project orphans are decided from this project's manifest alone.

## Goals / Non-Goals

**Goals:**

- Make the project cleanup pass delete only lockfile names that are not resolved with project scope.
- Leave `CleanOrphansByName`'s keep-set scan in place for the shared home tree.
- Replace the unit test that requires deleting a directory skmgr never locked.

**Non-Goals:**

- A scope or type field on `skmgr.lock`.
- Adopting unmanaged project directories into the lockfile or a new ledger.
- Changing `skmgr clean` (`cmd/clean.go` still removes non-reserved children of `.agents/`).
- Changing `install()`'s removal of the opposite type directory for the skill being copied. That path only touches the name being installed, in that skill's scope.
- Changing `cleanGlobalOrphans`.

## Decisions

### Diff the previous lock instead of listing the directory

After the install loop, where `Sync` calls `CleanOrphans` today, compute project removal candidates as:

- every `existingLock` entry name that is not among the resolved skills whose `EffectiveScope()` is project
- excluding `types.ReservedWorkflowNames()`

A nil lock or a lock with no entries yields no candidates. For each candidate, if `<project>/.agents/skills/<name>` is a directory, remove it; if `<project>/.agents/rules/<name>` is a directory, remove it. A missing path or a non-directory (`.openspec-target`, any other file) stays.

Do this after the install loop, not inside `removeDroppedEntries`. An install error then returns before any project directory is removed, and the caller does not write the new lock. `removeDroppedEntries` continues to unlink dropped names only.

`CleanOrphans` is the directory-listing keep-set. `Sync` must stop calling it. Delete `CleanOrphans` if the test is the only remaining caller, so the old scan cannot be wired back in by habit. Keep `CleanOrphansByName` for the global ledger union.

Alternatives considered:

- Pass the resolved names to `CleanOrphansByName` as the keep-set, and also add every on-disk name that is not in the lock. That preserves unmanaged directories but still treats the directory listing as the source of candidates, which is how the bug is structured.
- Adopt on-disk names the way `cleanGlobalOrphans` does. The global ledger is a keep-set, so adoption prevents deletion. `skmgr.lock` is the list of what the next sync may remove. Writing an unmanaged name into it would delete that directory on the following sync. A second project ledger would only restate "not in the lock."

### Treat "project scope" as the resolved set, not a lock field

Lock entries cannot be filtered by scope. A name in the previous lock is removed from the project trees when the manifest does not resolve it with project scope. That covers three cases with one rule:

- The name left the manifest: both trees lose that directory, which is what `TestInstall_RemovesOrphanWhenSkillDropped` already asserts.
- The name is still resolved with project scope: it was just installed, so it is not a candidate.
- The name moved to global scope: it is in the lock, it is not a project-scoped resolve, and the project directory is removed. `cleanupLinks` only unlinks the scope it is given, which on that run is global, so the old project symlink stays valid until the project directory is gone and the broken-link pass runs. Leaving the directory would keep agents on the stale project copy.

The cost is a name collision: an unmanaged project directory whose name equals some lock entry (including a global skill) is removed, because the lock cannot say which tree that name was installed into. Names that were never locked — the OpenSpec skills in the report — are not candidates. That is the bug.

### Tests replace the old orphan assertion

`TestCleanOrphans_RemovesUnlisted` expects `orphan-skill` to be deleted though it was never locked. Delete that test with `CleanOrphans`. Add coverage next to the existing command and engine tests (`cmd/install_test.go` helpers, `cmd/add` tests if they already drive `addCmd`, and `internal/engine/engine_global_test.go` for the home tree):

- `skmgr add` leaves `.agents/skills/other-tool-skill/` and `.agents/skills/.openspec-target` in place.
- `skmgr install` with no lockfile leaves an unmanaged skill directory in place, and leaves `.agents/rules/other-tool-rule/` in place.
- A previously locked `drop-me` is still removed (`TestInstall_RemovesOrphanWhenSkillDropped` stays).
- A skill installed in the project, then changed to global scope, loses `.agents/skills/<name>/` and is present under the home tree.
- `.agents/skills/skmgr-find-skill/` survives `install`.
- Existing global adoption tests stay green, including a hand-placed `~/.agents/skills/` directory and `TestSync_DroppedGlobalSkillIsStillPruned`.

### Docs

In `docs/flow.md`, replace the sentence that project orphans are decided from the manifest alone. Project cleanup removes previous lockfile names that are not resolved with project scope. Global cleanup stays a ledger union. The diagram node can keep its label.

## Risks / Trade-offs

- [A project that used sync to wipe extra skill directories will see them remain] → That is the fix. `skmgr clean` still clears the project store except reserved workflow names. The flow doc states the new rule.
- [An unmanaged project directory that shares a name with a lock entry is still deleted] → Accepted. The lock has one namespace and no scope. The reported deletions were names absent from `skmgr.lock`. Call this out in the flow doc in one sentence so it is not rediscovered as a new bug.
- [A scope change removes the project directory of a name that is still in the manifest] → Required so project symlinks do not keep serving the old tree. The home tree receives the install in the same sync.
- [Cleanup runs only after every install succeeds] → A failed install leaves dropped directories on disk with the old lock, and the next successful sync removes them. That is narrower than deleting first and then failing the install.

## Migration Plan

No lockfile or ledger migration. Directories already deleted are not restored. The next `add`, `install`, `update`, `sync`, or `remove` leaves unmanaged directories in place and still removes names the previous lock recorded that are no longer project installs. Rollback is reverting the sync cleanup change; lockfiles written by either version still read.
