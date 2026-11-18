# Spec Delta

## Purpose

Defines the machine-readable surface of the skmgr CLI that agent skills build on: one JSON document on stdout, a stable error shape, and no prompts. Without it, a skill would have to scrape tables meant for humans.

## ADDED Requirements

### Requirement: Mutating and planning commands accept --json
`init`, `update`, `add`, `remove`, `install`, `sync` and `clean` SHALL accept `--json`, and so SHALL `discover` and every `--dry-run` of those commands. The read-only commands `list`, `tree`, `verify`, `outdated` and `doctor` SHALL keep accepting it. With `--json`, stdout SHALL carry exactly one JSON document and nothing else.

#### Scenario: Single document on stdout
- **WHEN** `skmgr install --json` runs and logs progress at the default level
- **THEN** stdout parses as one JSON document and the progress lines appear only on stderr

#### Scenario: Flag accepted everywhere listed
- **WHEN** each listed command is invoked with `--json` against a valid project
- **THEN** none exits with usage status `2` for an unknown flag

### Requirement: Change results have a stable shape
For `add`, `remove`, `install`, `sync`, `update` and their dry runs, the document SHALL have `dry_run` (boolean) and `changes` (an array, never null). Each change SHALL have `action` (`install`, `update`, `reinstall` or `remove`), `name`, and, where known, `from_ref`, `from_sha`, `to_ref` and `to_sha`. Nothing to do returns `changes: []`. `init --json` SHALL report selected targets and files it would add, change, delete, or skip.

#### Scenario: Dry run of an update
- **WHEN** `skmgr update --dry-run --json` runs and one skill has a newer tag in range
- **THEN** the document has `dry_run: true` and one change with `action: "update"`, the skill's name, and distinct `from_ref` and `to_ref`

#### Scenario: Init reports files
- **WHEN** `skmgr init --targets cursor --dry-run --json` runs and the workflow files are missing
- **THEN** the document names target `cursor` and lists each missing skill and command file as an addition

#### Scenario: Nothing to do
- **WHEN** `skmgr install --json` runs against a project already in sync
- **THEN** the document has `dry_run: false` and `changes: []`

#### Scenario: Dry run writes nothing
- **WHEN** any command runs with `--dry-run --json`
- **THEN** the manifest, lockfile, installs and agent directories are byte-identical afterwards

### Requirement: Failures in JSON mode have a stable error shape
When a command run with `--json` fails, stdout SHALL carry an object with an `error` whose `code` is a stable snake_case identifier and whose `message` is human readable, plus an optional `fix`. The exit status SHALL be the same as without `--json`. Codes SHALL include `no_manifest`, `no_lockfile`, `invalid_manifest`, `hash_mismatch`, `network` and `usage`.

#### Scenario: No manifest
- **WHEN** `skmgr list --json` runs in a directory with no manifest
- **THEN** stdout is `{"error": {"code": "no_manifest", …}}` with a `fix` naming `skmgr init`, and the exit status is `1`

#### Scenario: Frozen install with drifted content
- **WHEN** `skmgr install --frozen --json` finds a fetched skill whose hash differs from the lockfile
- **THEN** the error code is `hash_mismatch` and the exit status is `3`

#### Scenario: Bad flag value
- **WHEN** `skmgr add <source> --scope sideways --json` runs
- **THEN** the error code is `usage`, the message lists the valid scopes, and the exit status is `2`

### Requirement: Existing machine-readable output is preserved
The JSON documents of `list`, `tree`, `verify`, `outdated` and `doctor` SHALL keep every field they have today with the same meaning. A `verify` or `doctor` report that finds drift SHALL still print its normal document, not an error object, and exit `3`. Exit codes `0` to `3` SHALL keep their documented meanings for every command.

#### Scenario: Verify drift
- **WHEN** `skmgr verify --json` finds a modified skill
- **THEN** stdout is the array of per-skill results including one with `status: "modified"`, and the exit status is `3`

#### Scenario: Additive fields only
- **WHEN** a new field is added to the `doctor` document
- **THEN** every existing field is still present with unchanged name and meaning

### Requirement: Only init may prompt
`skmgr init` is the only command that MAY read a selection from the terminal, and only for the agent question in `agent-workflow-files`. Every other command SHALL NOT read stdin or wait for confirmation, with or without `--json`. A command that needs a decision SHALL take it from a flag or fail with an error that names the flag.

#### Scenario: Closed stdin
- **WHEN** `skmgr install` runs with stdin closed
- **THEN** it completes or fails without blocking

#### Scenario: Missing decision
- **WHEN** a command other than init needs a value it was not given
- **THEN** it exits `2` and the message names the flag to pass
