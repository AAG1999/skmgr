# Design

## Context

See `proposal.md` for why, and the capability specs for what. This document covers how.

**What we copy from OpenSpec.** `openspec init` asks which tools to set up, writes a skill and a slash command per workflow into each tool's own directories, and prints a getting-started line plus "restart your IDE". The skill and the command share one body. The skill calls the CLI with `--json` for deterministic work. Dry-run and the real run share one write path. Generation is tool-agnostic content plus a small per-tool adapter.

**What we do not copy.** OpenSpec stores plans, specs, designs, tasks, and an archive as markdown. skmgr does not. The only file a person maintains is `skmgr.yml`. `skmgr.lock` is written by the CLI. There is no skill registry: find-skill searches the web itself, then `skmgr discover` inspects the repository the user picks.

**skmgr today.** `init` writes `skmgr.yml` and refuses to overwrite it. It does not prompt and does not write agent files. `update` re-resolves refs. `--json` exists on `list`, `tree`, `verify`, `outdated`, and `doctor`. Orphan cleanup deletes unknown directories under `.agents/skills`, and Gemini's project skills live in that directory.

## Goals / Non-Goals

**Goals:**
- One terminal step, `skmgr init`, then the four chat skills.
- Skill text and command text come from one template.
- The binary remains the only writer of the manifest, lockfile, canonical store, and links.
- A second `init` refreshes generated files through the same plan as a dry run.

**Non-Goals:**
- A trending index, catalog, or ranking API.
- Check, clean, or explore skills. `doctor`, `verify`, and `clean` stay CLI commands.
- Proposals, task lists, status files, or archives.
- Renaming `skmgr update`. It keeps re-resolving refs.
- Global-scope workflow files, or agents beyond the four built in.

## Decisions

### D1. Hybrid: skills drive the binary

The four skills call `skmgr … --json`. The Go code keeps resolving, hashing, atomic writes, link fallbacks, and the cache lock.

*Alternative: pure skills.* Rejected. The agent would clone, hash, and write the lockfile itself, and two agents could pin different commits for the same manifest.

### D2. Init is the picker and the refresher

`init` on a TTY asks a multi-select of the four agents, pre-selecting directories that already exist. `--targets` skips the question. No TTY and no `--targets` exits `2`.

A first run writes `skmgr.yml` (targets, empty skills) and the workflow files, then prints the four `/skmgr-*-skill` lines and a restart line. A later run updates `targets` and refreshes workflow files, and leaves the `skills` list and `skmgr.lock` alone. Refreshing files is not `skmgr update`, because that command already means "bump refs" and the chat skill `skmgr-update-skill` calls it.

*Alternative: a new `skmgr refresh` command.* Rejected. OpenSpec folds this into init and update; skmgr's `update` is already taken, so a second `init` is the one extra step.

### D3. Four skills, skill name and command name differ

| Action | Skill directory and chat name | Command |
|---|---|---|
| Find | `skmgr-find-skill` (`/skmgr-find-skill`) | `/skm-find` or `/skm:find` |
| Install | `skmgr-install-skill` | `/skm-install` or `/skm:install` |
| Update | `skmgr-update-skill` | `/skm-update` or `/skm:update` |
| Remove | `skmgr-remove-skill` | `/skm-remove` or `/skm:remove` |

The split is the same one OpenSpec uses (`openspec-propose` vs `/opsx-propose`) so Cursor and Copilot do not list the same action twice. Paths: `.cursor/commands/skm-find.md`, `.claude/commands/skm/find.md`, `.gemini/commands/skm/find.toml`, `.github/prompts/skm-find.prompt.md`. Skills use each agent's existing skills directory, so Gemini lands in `.agents/skills`.

Those four directory names are reserved in the manifest parser. `CleanOrphansByName` skips them, and `clean` removes `.agents` children except those names. An unmarked directory that happens to use a reserved name is left alone.

### D4. Find-skill searches; discover inspects

Find-skill asks the use case, searches the web and GitHub with the agent's own tools, and shows source URLs. Only an approved URL is passed to `skmgr discover`. Discover is `Fetch` plus a description reader. It requires a source, writes only to the download cache, and returns name, type, path, and ref. The skill then hands off to install-skill. Search results and `SKILL.md` text are untrusted data.

### D5. Ownership marker

Each generated file records `skmgr:generated`, a template-revision integer, and a SHA-256 of the file with the hash line removed. Init overwrites a file only when the hash still matches. `--force` overwrites edited files and still skips unmarked ones. The binary version is not in the marker, so a release that does not change templates does not rewrite committed files.

### D6. JSON

`cmd/output.go` emits one JSON document. Failures in JSON mode are `{"error": {code, message, fix?}}` with the same exit status as today. `add`, `remove`, `install`, `sync`, and `update` report `{dry_run, changes[]}` from `engine.Plan`. `init --json` reports selected targets and file actions (`add`, `change`, `delete`, `skip`), a different document from a skill change. `verify` and `doctor` keep printing their reports on exit `3`.

### D7. Permissions per skill

`find` may run `discover` and `list`. `install` may run `init`, `add`, and `install`. `update` may run `update` and `outdated`. `remove` may run `remove` and `list`. A blanket `skmgr *` permission is rejected because it would pre-approve `clean --all` from a find skill.

## Risks / Trade-offs

- **Find-skill can recommend a malicious repository** → it must show the URL and wait for approval before `discover` or `add`, and it treats fetched text as data.
- **Init now prompts, so scripts that ran `skmgr init` with a closed stdin will exit `2`** → the error names `--targets`, which is the migration.
- **Init used to refuse an existing manifest** → it now updates targets and files. The `skills` list is preserved, and a test locks that.
- **Gemini workflow files sit in the pruned tree** → reserved names, with tests that `install` and `clean` leave them.
- **Skill rules are prompt-level** → permission lists are the enforceable part.

## Migration Plan

1. Ship init's picker, the four generated files, `discover`, and `--json` together. Existing projects change when someone runs `skmgr init` again.
2. `skmgr update` behavior stays. Docs name `/skmgr-update-skill` for ref bumps and `skmgr init` for refreshing workflow files.
3. Rollback: drop an agent from the selection and run `skmgr init`. Unedited generated files for that agent are deleted. Declared skills stay.

## Open Questions

None. The search source, the four skill names, and the skill-plus-command layout are decided.
