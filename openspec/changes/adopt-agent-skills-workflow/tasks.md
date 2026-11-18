# Tasks

`skmgr update` is not renamed. Workflow-file refresh is `skmgr init`. Each group lands its own tests and docs.

## 1. JSON output (`cli-json-output`)

- [x] 1.1 Add `cmd/output.go` with a shared `--json` helper and a single-document emitter, and teach `Execute` to print `{"error": {code, message, fix?}}` in JSON mode with the existing exit status; verify with tests for `no_manifest`, `no_lockfile`, `invalid_manifest`, `hash_mismatch` (exit `3`), and `usage` (exit `2`).
- [x] 1.2 Add a `NetworkError` around git clone and fetch failures and map it to `network`; verify with an unreachable remote and an offline cache miss.
- [x] 1.3 Build `{dry_run, changes[]}` from `engine.Plan` for `add`, `remove`, `install`, `sync`, and `update`, with `changes` never null; verify install, update, reinstall, remove, and nothing-to-do, and that `--dry-run --json` leaves the manifest, lockfile, and installs byte-identical.
- [x] 1.4 Add `init --json` reporting selected targets and file actions `add`, `change`, `delete`, and `skip`; verify a dry run writes nothing. The generator fills the file list in task 3.3.
- [x] 1.5 Pin the existing `list`, `tree`, `verify`, `outdated`, and `doctor` JSON fields, including `verify` exiting `3` with its report, and document the contract in `docs/reference/cli.md`; verify the examples match real output.

## 2. Source discovery (`source-discovery`)

- [x] 2.1 Add the source root to `FetchResult` for git and local providers; verify existing provider tests still pass.
- [x] 2.2 Read `description` from `SKILL.md` frontmatter, strip control characters, and truncate with a flag; verify oversized text, terminal escapes, and missing frontmatter.
- [x] 2.3 Add `skmgr discover <source>` with `--ref`, `--path`, `--offline`, and `--json`, requiring a source and working without a manifest; verify skills directory, single-skill root, rules, version range, glob, nothing found (`no_skills_found`), and a missing source exiting `2`.
- [x] 2.4 Verify discover writes nothing outside the cache, honors offline mode, rejects option-like refs and escaping paths, and leaves a project's manifest, lock, and installs byte-identical.
- [x] 2.5 Round-trip every discovered entry through `skmgr add` and check the derived name; document `discover` in `docs/reference/cli.md` and `docs/flow.md`.

## 3. Init, generator, and protection (`agent-workflow-files`)

- [x] 3.1 Add the agent question to `skmgr init`: multi-select of the four agents, detected directories pre-selected, `--targets` skips it, no TTY and no `--targets` exits `2` naming the flag; verify the detected, explicit, unknown-target, and closed-stdin scenarios, and that an unknown target writes nothing.
- [x] 3.2 Make a second `init` update `targets` and leave the `skills` list and `skmgr.lock` unchanged; verify a manifest that already declares `pdf-tools`.
- [x] 3.3 Add `internal/workflow` with embedded templates, per-agent path adapters, the ownership marker (template revision plus content hash), and a plan of add, change, delete, and skip applied through `atomicfile`; verify the Cursor, Claude Code, Gemini, and Copilot paths, an idempotent second run, a dry run that writes nothing, and that an edited file is kept unless `--force`.
- [x] 3.4 Print the four `/skmgr-*-skill` lines and the IDE restart line after a successful init; verify the text for a Cursor selection.
- [x] 3.5 Reserve `skmgr-find-skill`, `skmgr-install-skill`, `skmgr-update-skill`, and `skmgr-remove-skill` in the manifest parser; verify the line-numbered error and add the case to `FuzzParseBytes`.
- [x] 3.6 Skip reserved names in `CleanOrphansByName` and stop `clean` from using `RemoveAll(.agents)`; verify `install` and `clean` leave Gemini and Cursor workflow files while removing installs and managed rule blocks.
- [x] 3.7 Delete generated files for agents no longer selected, keeping edited and unmarked files and unrelated files under `.github/` and `.claude/`; verify both cases.
- [x] 3.8 Add a `doctor` check for missing or stale workflow files whose remedy is `skmgr init`, ignoring edited files; verify missing, stale, edited, and current, including exit `3`.
- [x] 3.9 Document the picker, marker, paths, and cleanup protection in `docs/architecture.md`, and add the command adapter to `CONTRIBUTING.md`'s "Adding an agent" list.

## 4. The four skills (`workflow-skills`)

- [x] 4.1 Write the shared preamble (binary check, JSON, confirmation, project scope, exit codes, untrusted content, no plan or archive files) and the `skmgr-find-skill` template: ask the use case, search is the agent's, `discover` only after an approved URL, hand off to install; verify golden output and that its permissions exclude `add`, `remove`, `install`, `update`, and `init`.
- [x] 4.2 Write `skmgr-install-skill` (create a missing manifest with `skmgr init --targets`, then `add --dry-run` before `add`), `skmgr-update-skill` (all or named, `--latest` only when asked, no workflow-file refresh), and `skmgr-remove-skill`; verify the dry-run step precedes the write in each template.
- [x] 4.3 Generate the matching slash commands (`/skm-find` and `/skm:find`, and the same for install, update, remove) from the same body; verify skill and command text match for one agent.
- [x] 4.4 Add the golden test for all four actions and all four agents, the CLI-drift test against `Root()`, and a test that no template tells the agent to write a plan, task list, or archive; verify the drift test fails on a deliberately unknown flag.
- [x] 4.5 Write `docs/reference/workflow-skills.md` and update `README.md`, `docs/reference/agents.md`, `docs/flow.md`, `scripts/agent-install-prompt.md`, and `CHANGELOG.md` so the documented path is install, `skmgr init`, restart, then `/skmgr-find-skill`; verify the paths match the golden files and that ref bumps are still `skmgr update`.

## 5. Integration

- [x] 5.1 Add an end-to-end test: `init --targets` for all four agents, a second init is idempotent, `install` and `clean` leave workflow files, and an existing `skills` entry survives init; verify it on Linux, macOS, and Windows.
- [x] 5.2 Run `gofmt -l .`, `go vet ./...`, `go test ./...`, `golangci-lint run`, and `go run ./scripts/gen-dist`; verify completions and man pages include `discover` and that a GoReleaser snapshot still builds.
- [ ] 5.3 Manually invoke `/skmgr-find-skill`, `/skmgr-install-skill`, `/skmgr-update-skill`, and `/skmgr-remove-skill` in Cursor and one folder-namespaced agent (Claude Code or Gemini); verify the getting-started lines match what init printed.

## Workflow follow-up

- Archive this change after review, then verify the archived result and the main specs.
