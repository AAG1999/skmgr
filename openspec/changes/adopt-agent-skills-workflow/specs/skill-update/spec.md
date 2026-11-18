# Spec Delta

## Purpose

`skmgr update` keeps the meaning it has today: re-resolve skill refs and rewrite the lockfile. The chat skill `skmgr-update-skill` is a front end for that command. Updating installed skills does not refresh the generated workflow files.

## ADDED Requirements

### Requirement: Update re-resolves refs
`skmgr update [name...]` SHALL keep its current behavior. It re-resolves each skill against its declared ref, reinstalls what moved, rewrites `skmgr.lock`, and holds every unnamed skill at its locked commit when names are given. It SHALL accept `--latest`, `--offline`, and `--dry-run` with their current meanings, and `--json` as a change result. It SHALL NOT generate or modify workflow files.

#### Scenario: All skills
- **WHEN** `skmgr update` runs with no names and one skill has a newer tag inside its range
- **THEN** that skill moves and every other locked commit stays

#### Scenario: Named skills only
- **WHEN** `skmgr update pdf-tools` runs
- **THEN** only `pdf-tools` can change

#### Scenario: Range stays in range
- **WHEN** a skill has `ref: ^1.2.0`, the repository has `v1.2.4` and `v2.0.0`, and `skmgr update` runs
- **THEN** the lockfile records `v1.2.4`

#### Scenario: Latest crosses the range
- **WHEN** the same project runs `skmgr update --latest`
- **THEN** the lockfile records `v2.0.0` and the manifest `ref` is unchanged

#### Scenario: Workflow files untouched
- **WHEN** `skmgr update` runs and a workflow file is missing
- **THEN** the file is still missing afterwards

### Requirement: Update before any install fails as it does today
`skmgr update` with names and no `skmgr.lock` SHALL fail with the message it gives today.

#### Scenario: No lockfile
- **WHEN** `skmgr update pdf-tools` runs and no `skmgr.lock` exists
- **THEN** it fails and tells the user to install first
