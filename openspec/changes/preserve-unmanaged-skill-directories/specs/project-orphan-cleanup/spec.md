# Spec Delta

## Purpose

Defines which directories under a project's `.agents/` a sync may delete, so skills and rules that skmgr did not install survive `add`, `install`, `update`, `sync`, and `remove`.

## ADDED Requirements

### Requirement: Unmanaged project directories are kept
A sync SHALL leave a directory under the project's `.agents/skills/` or `.agents/rules/` in place when that directory's name is not an entry in the lockfile from before the sync. A missing lockfile has no such entries.

#### Scenario: Skill from another tool survives add
- **WHEN** `.agents/skills/other-tool-skill/` exists, that name is not in the lockfile, and the user runs `skmgr add` for a different skill
- **THEN** `.agents/skills/other-tool-skill/` is still present after the command

#### Scenario: Unmanaged rule directory survives install
- **WHEN** `.agents/rules/other-tool-rule/` exists, that name is not in the lockfile, and the user runs `skmgr install`
- **THEN** `.agents/rules/other-tool-rule/` is still present after the command

#### Scenario: First sync has no lockfile
- **WHEN** no lockfile exists, `.agents/skills/other-tool-skill/` exists, and the user runs `skmgr install`
- **THEN** `.agents/skills/other-tool-skill/` is still present after the command

### Requirement: Dropped lockfile names are removed
A sync SHALL delete each directory under the project's `.agents/skills/` and `.agents/rules/` whose name is an entry in the lockfile from before the sync and is not resolved from the manifest with project scope.

#### Scenario: Dropped skill directory is removed
- **WHEN** the lockfile from before the sync records `drop-me`, `.agents/skills/drop-me/` exists, and a later `skmgr install` resolves a manifest that does not include `drop-me`
- **THEN** `.agents/skills/drop-me/` is gone and each skill the manifest still resolves remains installed

#### Scenario: Dropped name is removed from the rules tree
- **WHEN** the lockfile from before the sync records `drop-me`, `.agents/rules/drop-me/` exists, and a later sync resolves a manifest that does not include `drop-me`
- **THEN** `.agents/rules/drop-me/` is gone

#### Scenario: Project copy is removed when scope becomes global
- **WHEN** a previous sync installed `moved` under the project's `.agents/skills/`, the manifest then resolves `moved` with global scope only, and the user syncs again
- **THEN** the project directory `.agents/skills/moved/` is gone and the skill is installed under the home `.agents/` tree

### Requirement: Project installs that remain are kept
A sync SHALL NOT delete a directory under the project's `.agents/skills/` or `.agents/rules/` whose name the manifest resolves with project scope.

#### Scenario: Installed skill stays
- **WHEN** the manifest resolves `keep-me` with project scope and `.agents/skills/keep-me/` exists
- **THEN** `.agents/skills/keep-me/` remains after sync

### Requirement: Non-directory entries are kept
A sync SHALL NOT delete a file or other non-directory entry under the project's `.agents/skills/` or `.agents/rules/`.

#### Scenario: Marker file survives add
- **WHEN** `.agents/skills/.openspec-target` is a file and the user runs `skmgr add` for a skill
- **THEN** `.agents/skills/.openspec-target` is still present after the command

### Requirement: Reserved workflow directories are kept
A sync SHALL NOT delete a project directory named `skmgr-find-skill`, `skmgr-install-skill`, `skmgr-update-skill`, `skmgr-remove-skill`, or `skmgr-create-skill`.

#### Scenario: Workflow skill survives install
- **WHEN** `.agents/skills/skmgr-find-skill/` exists and the user runs `skmgr install`
- **THEN** `.agents/skills/skmgr-find-skill/` is still present after the command

### Requirement: Shared home tree is not pruned as a project orphan
Syncing one project SHALL NOT delete a directory under the shared home `.agents/` tree when skmgr has no record of a project installing that name.

#### Scenario: Hand-placed global skill survives a project sync
- **WHEN** `~/.agents/skills/placed-by-hand/` exists, no project has installed that name, and the user syncs a project whose manifest does not include it
- **THEN** `~/.agents/skills/placed-by-hand/` is still present after the sync
