# Spec Delta

## Purpose

Defines how `skmgr init` asks which agents to set up, writes the four workflow skills and slash commands into those agents' directories, and how a later `init` refreshes those files. This is the one terminal step before the user works in chat.

## ADDED Requirements

### Requirement: Init asks which agents to set up
On a terminal, `skmgr init` SHALL ask which of `cursor`, `gemini`, `claude-code`, and `copilot` to set up, with detected agent directories pre-selected. It SHALL accept more than one. `--targets` SHALL skip the question and use that list, validated against the built-in agents. With no terminal and no `--targets`, it SHALL exit `2` and name `--targets`. It SHALL NOT prompt any command other than this question.

#### Scenario: Detected agents are pre-selected
- **WHEN** a person runs `skmgr init` in a project that contains `.cursor/` and selects `cursor` and `gemini`
- **THEN** `skmgr.yml` lists those two targets and no others

#### Scenario: Targets skip the question
- **WHEN** `skmgr init --targets copilot` runs in a project that contains `.cursor/`
- **THEN** it asks nothing and generates files only for `copilot`

#### Scenario: Unknown target
- **WHEN** `skmgr init --targets curser` runs
- **THEN** it exits `2`, names the known agents, and writes neither `skmgr.yml` nor any workflow file

#### Scenario: No terminal
- **WHEN** `skmgr init` runs with stdin closed and no `--targets`
- **THEN** it exits `2`, names `--targets`, and writes nothing

### Requirement: Init prints how to start
After a successful init, skmgr SHALL print one line for each generated skill, using that agent's invocation, and SHALL tell the user to restart the IDE so the skills load.

#### Scenario: Getting started lines
- **WHEN** init finishes for `cursor`
- **THEN** the output contains `/skmgr-find-skill`, `/skmgr-install-skill`, `/skmgr-update-skill`, `/skmgr-remove-skill`, and a line telling the user to restart the IDE

### Requirement: Files land in each agent's own directories
For each selected agent the generator SHALL write four skills and four slash commands. The skills are `skmgr-find-skill`, `skmgr-install-skill`, `skmgr-update-skill`, and `skmgr-remove-skill`. The commands are `find`, `install`, `update`, and `remove` under the `skm` prefix. Files SHALL be regular files, not links into the canonical store.

#### Scenario: Cursor layout
- **WHEN** workflow files are generated for `cursor`
- **THEN** skills exist at `.cursor/skills/skmgr-find-skill/SKILL.md` and the other three skill names, and commands exist at `.cursor/commands/skm-find.md` and the other three, invoked as `/skm-find`

#### Scenario: Claude Code layout
- **WHEN** workflow files are generated for `claude-code`
- **THEN** skills exist at `.claude/skills/skmgr-find-skill/SKILL.md` and commands at `.claude/commands/skm/find.md`, invoked as `/skm:find`

#### Scenario: Gemini layout
- **WHEN** workflow files are generated for `gemini` at project scope
- **THEN** skills exist at `.agents/skills/skmgr-find-skill/SKILL.md` and commands at `.gemini/commands/skm/find.toml`

#### Scenario: Copilot layout
- **WHEN** workflow files are generated for `copilot`
- **THEN** skills exist at `.github/skills/skmgr-find-skill/SKILL.md` and commands at `.github/prompts/skm-find.prompt.md`

### Requirement: Running init again refreshes workflow files
When `skmgr.yml` already exists, `skmgr init` SHALL update `targets` from the new selection and SHALL regenerate workflow files. It SHALL NOT change the `skills` list or `skmgr.lock`. Running it twice with the same selection and the same template revision SHALL change nothing the second time. `--dry-run` SHALL report each file it would add, change, or delete and SHALL write nothing. `--force` SHALL overwrite edited generated files.

#### Scenario: Skills list survives
- **WHEN** `skmgr.yml` already declares `pdf-tools` and the user runs `skmgr init --targets cursor`
- **THEN** `pdf-tools` is still declared, `targets` is `cursor`, and `skmgr.lock` is byte-identical

#### Scenario: Existing project gains the workflow
- **WHEN** a project has targets and no workflow files and the user runs `skmgr init` with those targets
- **THEN** the four skills and four commands exist for each target

#### Scenario: Dry run writes nothing
- **WHEN** `skmgr init --targets cursor --dry-run` runs and the skill files are missing
- **THEN** each missing file is listed and no file is created

### Requirement: Generated files are marked and user edits are preserved
Every generated file SHALL record that skmgr generated it, the template revision, and a hash of the generated content. The revision SHALL change only when that content changes. A later init SHALL overwrite a file only when its content still matches the hash. A changed file SHALL be left and reported as edited, unless `--force` is given. A file with no skmgr marker SHALL be left even with `--force`.

#### Scenario: Unedited file is refreshed
- **WHEN** a newer template revision is current and a generated skill still matches its hash
- **THEN** init rewrites that file

#### Scenario: Edited file is kept
- **WHEN** the user has changed `.cursor/skills/skmgr-find-skill/SKILL.md` and runs init without `--force`
- **THEN** that file is unchanged and listed as edited, and the other files are still refreshed

#### Scenario: Unmarked file is kept
- **WHEN** a file at a generated path has no skmgr marker
- **THEN** init leaves it alone and reports it as not managed

### Requirement: Files for agents no longer selected are removed
When an agent is no longer selected, init SHALL delete that agent's generated files that still match their hash, and SHALL remove directories it leaves empty. It SHALL NOT delete edited or unmarked files, and SHALL NOT delete anything outside the generated paths.

#### Scenario: Agent dropped
- **WHEN** `copilot` is removed from the selection and init runs
- **THEN** the unedited `skmgr-*-skill` directories and `skm-*` prompts under `.github/` are deleted and the rest of `.github/` remains

#### Scenario: Edited file survives
- **WHEN** the same removal happens but one generated skill was edited
- **THEN** that skill remains and is listed as left alone

### Requirement: Workflow files survive skmgr's own cleanup
Orphan cleanup during `install`, `sync`, `add`, `remove`, and `update`, and `skmgr clean` in any mode, SHALL NOT delete generated workflow files, including Gemini's files under `.agents/skills/`.

#### Scenario: Install keeps Gemini workflow skills
- **WHEN** `gemini` is a target and `skmgr install` runs
- **THEN** `.agents/skills/skmgr-find-skill/` and the other three skill directories still exist

#### Scenario: Clean keeps workflow files
- **WHEN** `skmgr clean` runs in a project with workflow files for `gemini` and `cursor`
- **THEN** installed skills and managed rule blocks are removed, and every workflow skill and command file still exists

### Requirement: Workflow skill names are reserved
The manifest parser SHALL reject a skill or rule whose effective name is `skmgr-find-skill`, `skmgr-install-skill`, `skmgr-update-skill`, or `skmgr-remove-skill`, with an error naming the line and the reserved name.

#### Scenario: Collision is rejected
- **WHEN** `skmgr.yml` declares a skill whose derived name is `skmgr-find-skill`
- **THEN** parsing fails with the line number, the name, and a hint to set a different `name:`

### Requirement: Doctor reports missing or stale workflow files
`skmgr doctor` SHALL check workflow files for every manifest target. It SHALL fail when an expected file is missing or was generated from an older template revision, and its remedy SHALL be to run `skmgr init` again. A file reported as edited SHALL NOT fail the check.

#### Scenario: Missing file
- **WHEN** a target lacks one workflow skill and `skmgr doctor` runs
- **THEN** the check fails, doctor exits `3`, and the remedy says to run `skmgr init`

#### Scenario: Current files pass
- **WHEN** every expected file exists and carries the running template revision
- **THEN** the check passes
