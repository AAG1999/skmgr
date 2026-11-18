# Spec Delta

## Purpose

Lets a person or agent see which skills and rules one chosen source offers, and the exact `path` and `ref` to add them with, before changing anything. `skmgr-find-skill` uses it after the user picks a repository. Discover does not search the web or rank trending skills.

## ADDED Requirements

### Requirement: Discover lists what a source offers
`skmgr discover <source>` SHALL list every skill and rule the source offers, using the same discovery rules `install` applies to that source. Each result SHALL carry its derived `name`, its `type` (`skill` or `rule`), its `path` relative to the source root, and its `description` when its `SKILL.md` frontmatter has one. The output SHALL also state the `ref` requested and the `commit_sha` it resolved to, where the source is git.

#### Scenario: Repository with a skills directory
- **WHEN** `skmgr discover https://example.test/skills.git --json` runs against a repository with `skills/pdf-tools/SKILL.md` and `skills/skill-creator/SKILL.md`
- **THEN** the document lists two results with paths `skills/pdf-tools` and `skills/skill-creator`, type `skill`, and a `commit_sha`

#### Scenario: Single-skill repository
- **WHEN** the source has a `SKILL.md` at its root and no `skills/` directory
- **THEN** there is one result whose `path` is empty and whose `name` is the repository name

#### Scenario: Description from frontmatter
- **WHEN** a skill's `SKILL.md` frontmatter has `description: Fills PDF forms`
- **THEN** the result's `description` is `Fills PDF forms`

#### Scenario: Nothing found
- **WHEN** the source has neither a `skills/` or `rules/` directory nor a root `SKILL.md` or `AGENTS.md`
- **THEN** discover exits `1` with the same guidance install gives and, in JSON mode, the error code `no_skills_found`

### Requirement: Results round-trip into add
For every result, passing its `path` as `--path`, its `type` as `--type`, and the same `source` and `ref` to `skmgr add` SHALL select exactly that skill or rule and give it the same derived `name` discover reported.

#### Scenario: Round trip
- **WHEN** discover reports `{name: "pdf-tools", path: "skills/pdf-tools"}` and the user runs `skmgr add <source> --path skills/pdf-tools`
- **THEN** the manifest gains one entry that installs and is named `pdf-tools`

### Requirement: Discover narrows by path and ref
`--path` SHALL limit results to entries under that path, including a glob such as `skills/*`. `--ref` SHALL discover against that branch, tag, commit or version range, resolved exactly as `add` resolves it.

#### Scenario: Version range
- **WHEN** `skmgr discover <source> --ref '^1.2.0'` runs and the repository has tags `v1.2.0` and `v1.3.1`
- **THEN** results are read from `v1.3.1` and the output states `resolved_ref` as `v1.3.1`

#### Scenario: Path glob
- **WHEN** `--path 'skills/p*'` is given
- **THEN** only entries whose path matches are listed

### Requirement: Discover never modifies the project
Discover SHALL write only to the download cache. It SHALL NOT create or change `skmgr.yml`, `skmgr.lock`, `.agents/`, agent directories, or the global ledger, and SHALL work in a directory with no manifest.

#### Scenario: Outside a project
- **WHEN** `skmgr discover <source>` runs in an empty directory
- **THEN** it succeeds and the directory is still empty afterwards

#### Scenario: Inside a project
- **WHEN** it runs in a project with a manifest and lockfile
- **THEN** both files and the installed tree are byte-identical afterwards

### Requirement: Discover honours offline mode
`--offline`, `SKMGR_OFFLINE` and the configured offline setting SHALL forbid network access. A git source that is not already cached SHALL fail with an error that names the flag rather than fetching.

#### Scenario: Uncached source offline
- **WHEN** `skmgr discover <source> --offline` runs and the source has never been fetched
- **THEN** it exits `1` with `network` as the JSON error code and makes no network call

### Requirement: Discover validates input and bounds untrusted text
Discover SHALL apply the same checks to `source`, `--ref` and `--path` as `add`, rejecting arguments git would read as options and paths that escape the source root. Descriptions SHALL have control characters removed and SHALL be truncated to a fixed maximum length, with an indication when truncated.

#### Scenario: Option-like ref
- **WHEN** `skmgr discover <source> --ref=--upload-pack=x` runs
- **THEN** it exits `2` without invoking git

#### Scenario: Escaping path
- **WHEN** `--path ../../etc` is given
- **THEN** it fails and names the path as outside the source

#### Scenario: Oversized description
- **WHEN** a `SKILL.md` description is much longer than the maximum or contains terminal escape sequences
- **THEN** the reported description is truncated, flagged as truncated, and contains no control characters

### Requirement: Discover inspects one source
`skmgr discover` SHALL require a source argument. It SHALL NOT search the web, accept a use-case query, or rank results by popularity.

#### Scenario: Missing source
- **WHEN** `skmgr discover` runs with no source
- **THEN** it exits `2` and names the source argument
