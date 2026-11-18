# Proposal

## Why

After `skmgr` is installed, day-to-day use should happen in the agent's chat, the way OpenSpec does. A person runs `skmgr init` once, picks the agents they use, and from then on finds, installs, updates, and removes skills with `/skmgr-find-skill` and its siblings. The binary keeps the work that must be exact: resolving, fetching, hashing, writing `skmgr.yml` and `skmgr.lock`, and linking.

## What Changes

- `skmgr init` asks which agents to set up when a person is at the terminal, the way `openspec init` asks which tools. It writes `skmgr.yml` with those targets and generates a skill plus a slash command for each of find, install, update, and remove. It then prints a one-line invocation for each skill and tells the user to restart the IDE.
- `--targets` skips the questions, so CI and scripts stay non-interactive. A terminal that is not a TTY and has no `--targets` fails and names the flag.
- Running `skmgr init` again pre-selects the current targets, updates that list, and refreshes generated files the user has not edited. It does not touch declared skills or `skmgr.lock`. `skmgr update` keeps its current meaning: re-resolve refs and rewrite the lockfile. There is no rename to `upgrade`.
- Four workflow skills, each with a matching slash command: `skmgr-find-skill`, `skmgr-install-skill`, `skmgr-update-skill`, `skmgr-remove-skill`.
- `skmgr discover <source>` lists what a repository offers. Find-skill uses it after the user picks a repository the agent found. Discover does not search the web or rank "trending" skills.
- Mutating commands and `discover` accept `--json` and, except for the `init` prompt, never ask questions.
- Generated files carry an ownership marker. Orphan cleanup, `clean`, and the manifest parser leave them alone.
- The only project file a person maintains is `skmgr.yml`. Skills do not create plans, task lists, status notes, or an archive.

Out of scope: a skill registry or trending ranking inside skmgr; check, clean, and explore skills (the `doctor`, `verify`, and `clean` commands stay on the CLI); pure-skill operation with no binary; global-scope workflow files; agents beyond the four built in; any change to lockfile format, cache layout, or the hash and link algorithms.

## Capabilities

### New Capabilities

- `agent-workflow-files`: Interactive `init`, the four generated skills and slash commands, and refreshing them by running `init` again.
- `workflow-skills`: What find, install, update, and remove do in chat, including the web search find-skill performs and the confirmation the others require.
- `cli-json-output`: Machine-readable output for the commands those skills call, and the rule that only `init` may prompt.
- `source-discovery`: Listing the skills and rules inside one chosen source, without searching the web.
- `skill-update`: `skmgr update` keeps re-resolving refs, for every skill or for the names given, and does not refresh workflow files.

### Modified Capabilities

- None. The `lockfile` spec is unchanged.

## Impact

- **New code**: `internal/workflow` (templates and per-agent adapters), `cmd/discover.go`, and an interactive picker in `cmd/init.go`.
- **Changed code**: `cmd/init.go`, `cmd/update.go` (`--json` only; behavior unchanged), `cmd/{add,remove,install,sync,clean,doctor}.go` and `cmd/dryrun.go` for `--json`, `internal/types/config.go` (command paths), `internal/engine/installer.go` and `cmd/clean.go` (keep generated files), `internal/manifest` (reserved workflow names).
- **CLI compatibility**: `update` does not change meaning. Existing `--json` shapes of `list`, `tree`, `verify`, `outdated`, and `doctor` are preserved. Exit codes `0`–`3` are unchanged.
- **Docs**: `README.md`, `docs/reference/cli.md`, `docs/reference/agents.md`, `docs/flow.md`, `docs/architecture.md`, a new `docs/reference/workflow-skills.md`, `scripts/agent-install-prompt.md`, and `CHANGELOG.md`.
- **Dependencies**: none new. Templates use `go:embed`. The picker uses the standard library.
