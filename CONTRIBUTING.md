# Contributing to skmgr

Thanks for wanting to help. This file covers how to get set up, what the code
review looks for, and the few conventions that are not obvious from reading the
source.

## Getting set up

You need Go 1.22 or newer and `git` on your `PATH`. That is the whole list —
`skmgr` shells out to the system `git` rather than embedding one, so whatever
credentials and proxy settings already work for you work here too.

On macOS 26 (Tahoe) use Go 1.24 or newer. Earlier versions do not write an
`LC_UUID` into the binary, and dyld there refuses to load one without it, so
`go build` produces an executable that aborts before it starts. If you must use
an older Go, build with `-ldflags="-B gobuildid"`, which is what CI does.

```bash
git clone https://github.com/AAG1999/skmgr.git
cd skmgr
go build -o skmgr .
go test ./...
```

The test suite creates real git repositories in temporary directories, so it
needs `git` but never the network.

### Before you open a pull request

```bash
gofmt -l .          # must print nothing
go vet ./...
go test ./...
golangci-lint run   # matches the lint job in CI
```

## How the project is laid out

| Directory | What lives there |
| --- | --- |
| `cmd/` | One file per CLI command. Cobra wiring, flags, and output formatting. |
| `internal/manifest/` | Reading, validating and writing `skmgr.yml`. |
| `internal/lockfile/` | Reading and writing `skmgr.lock`, plus directory hashing. |
| `internal/provider/` | Fetching sources: git, local paths, the cache. |
| `internal/engine/` | Resolving the manifest, installing, planning a dry run. |
| `internal/linker/` | Symlinks, junctions, and merging rules into shared files. |
| `internal/types/` | Shared structs and the agent definitions. |
| `internal/config/` | User-level settings from the config file and environment. |
| `schemas/` | The JSON Schema shipped for editor autocomplete. |

[docs/architecture.md](docs/architecture.md) explains how these fit together, and
[docs/flow.md](docs/flow.md) traces what happens during each command. Both are worth
reading before a change that crosses package boundaries.

## What review looks for

### Tests

Every behavior change needs a test. What matters is not coverage but whether
the test would have caught the bug: a test that asserts the code does what it
does, rather than what it should do, is worse than nothing because it makes the
next person afraid to change it.

Name tests after the property, not the function.
`TestVerify_DetectsModifiedContent` says what breaks if it fails;
`TestVerify2` does not.

If you are fixing a bug, write the failing test first and put it in the commit.
It is the only durable proof the bug is gone.

### Comments

Comment the *why*, never the *what*. If a line needs explaining, the comment
should say what constraint forced it, what breaks without it, or what was tried
first. Anything the code already says is noise the moment someone edits the
code and not the comment.

```go
// Bad: increments the counter
count++

// Good: the lockfile is read again rather than reused, because a concurrent
// skmgr run may have rewritten it while we were fetching.
```

### Error messages

An error is read by someone who does not know the codebase. Say what failed,
what value caused it, and what to do next.

```
manifest validation: line 10: skills[1].targets has unknown agent "curser"
(known: cursor, gemini, claude-code, copilot); did you mean "cursor"?
```

Where a manifest is at fault, include the line number — `internal/manifest`
already has the machinery.

### Things that need extra care

Some parts of this codebase can destroy a user's work, so changes there get
looked at harder:

- **Anything that deletes.** Orphan cleanup, `clean`, and cache pruning have all
  caused data loss before. A change here needs a test proving the *other*
  things survive.
- **Global scope.** `~/.agents/` is shared by every project on the machine.
  Pruning it against a single project's manifest is how `skmgr` once deleted
  every globally installed skill.
- **Path handling.** A `name` or `path` from a manifest becomes a filesystem
  path. Anything joining one onto a directory needs a containment check;
  `resolveWithinBase` and `types.ValidateName` exist for this.
- **Arguments passed to git.** git reads a leading `-` as an option, and some
  options run a command. `types.ValidateGitArgument` guards this.
- **Writes that must not tear.** Manifest, lockfile and merged rule files are
  written through `internal/util/atomicfile` so an interrupted run cannot leave
  half a file.

## Adding an agent

Agent definitions are Go code in `internal/types/config.go` rather than
configuration, deliberately: a manifest naming an agent that only exists on the
author's machine would install for them and fail for everyone else, which
defeats the point of committing a lockfile.

Adding one means:

1. A definition in `internal/types/config.go` describing its skills directory,
   its rules path, and whether rules are symlinked or merged.
2. Its name in `SupportedAgentNames()`.
3. Its name in the `agent` enum in `schemas/skmgr.schema.json` — a test asserts
   these two agree.
4. Detection in `cmd/init.go`, so `skmgr init` picks it up from a directory
   that already exists.
5. A row in the agent table in `README.md`.

## Commits and pull requests

Commit messages use the conventional prefixes: `feat:`, `fix:`, `docs:`,
`test:`, `refactor:`, `chore:`. The subject should say what changed and the
body why.

Keep a pull request to one idea. A refactor bundled with a behavior change is
hard to review and harder to revert.

If your change is user-visible, add an entry to `CHANGELOG.md` under
`[Unreleased]`, in the section that fits (`Added`, `Changed`, `Fixed`,
`Security`). Write it for someone upgrading who has not read the diff.

## CI

| Workflow | When | What it does |
| --- | --- | --- |
| `ci.yml` | Every push and pull request | Lint, tests on Linux, macOS and Windows, CLI scenarios, a GoReleaser snapshot. |
| `integration.yml` | Every push and pull request | End-to-end lifecycle against real repositories, including a pinned public commit. |
| `install-script.yml` | Changes to `scripts/install.sh` or `scripts/install.ps1` | Syntax checks; on Windows, a mocked end-to-end run of `install.ps1` against a locally built zip. |
| `security.yml` | Every push, plus weekly | `govulncheck` and `gosec`. |
| `nightly.yml` | Daily | Fuzzing with a carried-over corpus, and benchmarks. |
| `release.yml` | A `v*` tag | Builds, packages and publishes. |

You can run the fuzzers yourself:

```bash
go test ./internal/manifest -run FuzzParseBytes -fuzz FuzzParseBytes -fuzztime 1m
go test ./internal/lockfile -run FuzzRead -fuzz FuzzRead -fuzztime 1m
```

A crash writes the offending input to `testdata/fuzz/`. Commit it — it becomes
a regression test that runs on every build from then on.

## Releasing

Releases are cut by pushing a tag:

```bash
git tag v1.2.0
git push origin v1.2.0
```

GoReleaser builds the binaries, generates the shell completions and man pages
from the live command tree, and attaches them along with the JSON schema and
the bootstrap scripts (`scripts/install.sh`, `scripts/install.ps1`). The
`release-config` job in `ci.yml` runs a snapshot build on every commit, so a
broken release config is caught before the tag rather than during it.

## Reporting a security issue

Please do not open a public issue for a vulnerability. Email
a.a.gawade1999@gmail.com with the details and a way to reproduce it.

## Licence

Contributions are accepted under the [Apache 2.0](LICENSE) licence. Every new
source file starts with the standard header; copy it from any existing file.
