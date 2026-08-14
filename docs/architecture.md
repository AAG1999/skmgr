# Architecture

System design of **skmgr** — a CLI package manager for AI agent skills and rules.

## Overview

skmgr reads a declarative manifest (`skmgr.yml`), resolves sources (git or local), installs into a canonical store (`.agents/`), then links into agent-native directories. A lockfile (`skmgr.lock`) records resolved commit SHAs and content hashes.

```
┌─────────────────────────────────────────────────────────────┐
│                         skmgr CLI                           │
│  init · add · remove · install · update · list · sync · clean│
│  verify · outdated · tree · doctor · cache · self-update      │
└────────────────────────────┬────────────────────────────────┘
                             │
                    ┌────────┴────────┐
                    │     Config      │
                    │ file · env vars │
                    └────────┬────────┘
                             │
                             ▼
┌─────────────────────────────────────────────────────────────┐
│                          Engine                             │
│              resolve → install → link → cleanup             │
│                  (or plan, for --dry-run)                   │
└───────┬─────────────────────┬───────────────────┬───────────┘
        │                     │                   │
        ▼                     ▼                   ▼
┌───────────────┐    ┌────────────────┐   ┌────────────────┐
│   Provider    │    │   Installer    │   │     Linker     │
│ git · local   │    │ copy to        │   │ symlink · merge│
│               │    │ .agents/       │   │ · gitignore    │
└───────┬───────┘    └───────┬────────┘   └───────┬────────┘
        │                    │                    │
        ▼                    ▼                    ▼
 ~/.skmgr/cache/         .agents/          Agent dirs
 ~/.skmgr/global.lock    skmgr.lock        (.cursor, …)
```

## Components

| Layer | Path | Responsibility |
|-------|------|----------------|
| Entry | `main.go` | Starts CLI |
| CLI | `cmd/` | Cobra commands and flags |
| Engine | `internal/engine/` | Orchestrates sync: resolve, install, link, orphan cleanup |
| Provider | `internal/provider/` | Fetch sources (`GitProvider`, `LocalProvider`) |
| Linker | `internal/linker/` | Symlinks, rule merge, managed `.gitignore` |
| Manifest | `internal/manifest/` | Parse / validate / write `skmgr.yml`, derive names |
| Lockfile | `internal/lockfile/` | Read / write `skmgr.lock` and `~/.skmgr/global.lock`, directory hashing |
| Semver | `internal/semver/` | Range detection and best-match tag selection |
| Types | `internal/types/` | Domain models, agent registry (`AgentDef`), name validation |
| Log | `internal/log/` | Leveled logger on stderr, driven by the global flags |
| Config | `internal/config/` | Resolves the config file and env vars into paths and defaults |
| Atomic writer | `internal/util/atomicfile/` | Write-to-temp-then-rename for every file skmgr owns |
| File lock | `internal/util/lock/` | Cross-platform advisory lock guarding the shared cache |

## Data artifacts

| Artifact | Location | Role |
|----------|----------|------|
| Manifest | `skmgr.yml` or `skmgr.yaml` | Declared dependencies (source of truth for intent) |
| Lockfile | `skmgr.lock` | Resolved SHAs, resolved tags, content hashes, and the source/path/ref each entry came from |
| Cache | `~/.skmgr/cache/git/<source-hash>/` | One clone plus one immutable tree per commit |
| Global ledger | `~/.skmgr/global.lock` | Which project owns which global skill |
| Canonical store | `.agents/` or `~/.agents/` | Installed skills/rules before linking |
| Agent views | `.cursor/`, `.claude/`, `.gemini/`, `.github/` | Symlinks or merged rule files |

### Global ledger

`~/.agents/` is shared by every project on the machine, so orphan cleanup there
cannot be decided from a single manifest. The ledger records, per project root,
the global skill names that project declares. Before pruning, the engine writes
the current project's list and then prunes against the **union** of all projects.

The ledger also carries an `adopted` list. When it is created for the first time
(an upgrade from a version that had no ledger), any global skills already on disk
are adopted so they are never mistaken for orphans. `skmgr clean --global` is the
deliberate way to release them.

### Identity: derived names, source-and-path keys

A skill's `name` is a local alias, so it is derived at parse time rather than
demanded from the user: the last segment of `path`, or the repository name when
there is no path. An explicit `name` overrides the derivation, and two entries
deriving the same name is an error.

Names cannot be the identity used for pinning, because one wildcard manifest
entry expands into many installs whose names are generated during resolution.
The lockfile therefore records `source` and `path` on every entry, and
`--frozen` and `update <name>` key their pin map on that pair. Names remain the
key for everything user-facing: the canonical directory, the symlink, the
merged-block delimiter, and the lockfile entry.

### Cache layout

A source repository gets one directory, keyed by a hash of its URL:

```
~/.skmgr/cache/git/<source-hash>/
    .lock            advisory lock held across clone and fetch
    repo/            the clone, deliberately never checked out
    commits/<sha>/   one extracted tree per commit, immutable once published
```

Trees are extracted with `git archive` into a staging directory and renamed into
place, so an interrupted run cannot leave a partial tree that a later run reads
as a cache hit. Because a commit directory never changes after it is published,
two runs wanting different refs of the same repository cannot disturb each
other, and reads need no lock at all. The clone itself is shared mutable state,
so cloning and fetching happen under `.lock`.

Earlier versions checked the requested ref out inside a single shared clone per
source. Those directories cannot be read by the current code, so the first run
after upgrading deletes them; the content is re-fetchable and the only cost is
one slower run.

### Durability

Every file skmgr owns is written through `internal/util/atomicfile`: content
goes to a temporary file in the destination directory, is flushed, and is then
renamed over the target. A crash mid-write leaves either the old file or the new
one, never a truncated manifest, lockfile, or half-scrubbed rule file.

Installs get the same treatment at directory granularity. The fetched tree is
copied into a staging directory under the canonical store and renamed into place
only once the copy has fully succeeded, so a failure part-way through leaves the
previously installed version untouched rather than a partial skill.

Under `--frozen`, the staged content is hashed and compared to the lockfile
*before* the rename. Content that does not match what was locked is rejected
while the previous install is still in place, which is what makes a frozen
install safe to run against a source that has drifted.

### Configuration layering

`internal/config` resolves four inputs into one answer, from most to least
specific: a command-line flag, an environment variable, `config.yml`, then the
built-in default. Flags are applied by the commands themselves; everything below
that line is resolved in one place, so there is a single file to look at when a
setting does not take effect.

Only the parsed file is cached. The environment is re-read on each lookup,
because a process that changes `HOME` — a test, or a wrapper script — expects the
paths to follow. Caching the fully resolved config instead would freeze the
first answer for the life of the process.

A config file that cannot be parsed does not fall back silently. Path lookups
still return defaults so nothing panics, but the error is surfaced before any
command runs, because acting on settings the user believes are in effect is
worse than refusing.

### Planning versus applying

`--dry-run` is served by `Engine.Plan`, which shares its resolver options with
`Engine.Sync` rather than reimplementing them. That sharing is the point: a plan
computed under different rules than the run it predicts is worse than no plan.

Resolution still happens during a plan — there is no way to know whether a
branch has moved without asking — but it writes only to the download cache,
never to the project, the lockfile, or the manifest. Classification then
compares the resolved commit against the lockfile and the installed content
against its recorded hash, which is what separates *update* (the source moved)
from *reinstall* (the source is the same but the files on disk are not).

### Ref semantics

Whether the cache can be trusted depends on the kind of ref:

| Ref kind | Treatment |
|----------|-----------|
| Branch, or unset (remote default branch) | Moving. Fetch, then check out `origin/<branch>` so a cached clone advances. |
| Semver range | Moving within bounds. Fetch, pick the best matching tag, then treat that tag as immutable. |
| Tag, commit SHA | Immutable. Reuse the cached checkout and skip the network once present. |

This split is what makes `--offline` and `--frozen` meaningful: both force the
immutable path for everything, and fail rather than fetch.

### Versioning without a registry

There is no package registry, so a git tag is the only published-version signal
available. `internal/semver` therefore does two separable jobs:

- **Range detection.** Deciding whether a `ref` is a constraint at all. This is
  deliberately conservative — only `latest`, a leading operator, a version
  wildcard, or a compound constraint counts — because `ref` is equally allowed
  to be a branch name, and reading `release-2` as a range would install
  something nobody asked for. A bare `v1.2.3` is an exact tag.
- **Best-match selection.** Given the repository's tags and a constraint, the
  newest tag that satisfies it. Tags that are not semantic versions are ignored,
  and prereleases are excluded unless the constraint names one.

Tags are read from the cached clone rather than from `git ls-remote`. The clone
is already fetched with `--tags` on every run, so a local read is exactly as
current, costs no extra round trip, needs no freshness heuristic, and is the
only option that works under `--offline`.

Resolution happens inside the cache layer, immediately after the fetch and
before a commit is chosen, so everything downstream sees one concrete commit and
neither knows nor cares that a range was involved. The tag that was selected is
recorded in the lockfile as `resolved_ref`, which is what keeps a range
reproducible: the manifest says what was asked for, the lockfile says what that
meant at the time.

## Design patterns

- **Package manager model** — manifest + lockfile + cache + install tree
- **Canonical store + projection** — one copy under `.agents/`; agents get views via link/merge
- **Strategy / registry** — providers via interface; agents via `AgentDef`
- **Single binary CLI** — classic Go layout (`main` + `cmd` + `internal`)

## Agent model

Each agent is an `AgentDef` with:

- Skills directory path (project / global)
- Rules path (directory or single file)
- Rule strategy: `symlink` | `merge` | `skip`
- Whether it already reads `.agents/` natively (no skill symlink needed)

Built-in agents: `cursor`, `gemini`, `claude-code`, `copilot`.

## Scopes

| Scope | Canonical root | Agent views |
|-------|----------------|-------------|
| `project` (default) | `<project>/.agents/` | `<project>/.cursor/`, `<project>/.claude/`, … |
| `global` | `~/.agents/` | `~/.cursor/`, `~/.claude/`, … |

Scope decides the root for **both** the canonical store and the agent views. A
global install never writes into the current project directory, whatever the
working directory happens to be.

Scope is a property of the manifest entry, not of the invocation. `install`,
`sync`, and `update` therefore have no `--global` flag: what a checkout installs
must not depend on how the command was typed, or two people running the same
repository would end up with different layouts. `--global` exists only on `add`
(which is choosing a scope for a new entry), `list` (a filter), and `clean` (a
target), and the other commands reject it rather than accepting and ignoring it.

## Skill vs rule

| Type | Stored as | Linked as |
|------|-----------|-----------|
| `skill` | Directory under `.agents/skills/` | Directory symlink into agent skills dir |
| `rule` | Directory under `.agents/rules/` | Symlink into rules dir, or merge into one file with `<!-- skmgr:start:name -->` delimiters |

A rule is stored as a directory like a skill; the content to merge is read from
the first of `AGENTS.md`, `SKILL.md`, `rule.md` or `README.md` found inside it.

Because the two types share a namespace but not a directory, changing a
dependency's `type` would leave the old copy under the other tree, where
name-keyed orphan cleanup still considers it valid. Installing therefore removes
the same name from the opposite tree first.

### Managed block safety

Merged rules and the managed `.gitignore` block are rewritten in place. Before
any rewrite, the delimiters are checked for balance. An unmatched start or end
marker aborts the write with an error naming the file, because rewriting such a
file would silently discard everything after the orphaned marker.

Because a merged rule leaves no symlink behind, the broken-link sweep cannot find
it. `Sync` therefore diffs the previous lockfile against the resolved set and
explicitly unlinks anything that disappeared.

The same blindness applies to a skill that merely lost a target. A skill's
effective target list can be narrower than the manifest's, so before relinking,
`Sync` unlinks the name from **every** known agent and then recreates only the
links it should have. `skmgr clean` uses the same idea in bulk via
`Linker.StripManagedRules`, which removes every `skmgr:` block from every merged
file.

The managed `.gitignore` block is regenerated wholesale on each run, so it is
labelled as generated: content written inside it by hand is discarded rather
than half-preserved.

### Input safety

Skill names and source paths come from user-authored YAML and are joined onto
install directories, so both are validated before use:

- Names must be a single path component: no separators, no `..`, no characters
  that are illegal in a directory name, and no Windows reserved device names.
  Checked at parse time and again at install time.
- A `path:` is resolved and required to stay inside the source root; glob matches
  are re-checked after expansion.
- Symlinks inside a skill are recreated rather than dereferenced, but only if
  they resolve inside that skill. A link escaping it aborts the install.

The manifest parser rejects unknown keys, unknown agent names, and skill names
that differ only in case. Each of these used to pass silently and fail later, at
a point far from the typo: a misspelled `pth:` installed the wrong directory, an
agent typo installed nothing, and two case-variant names collided into one
directory on macOS and Windows.

### Link portability

Links are written relative to the directory holding them, computed with
`filepath.Rel`. An absolute link breaks as soon as the project is moved,
renamed, or mounted at a different path inside a container, which is routine.
Links found in the absolute form are rewritten on the next sync, so a project
converges without anyone intervening.

That makes resolving a link's target a two-step operation, and getting it wrong
is destructive: a relative target resolved against the process working directory
looks broken from anywhere but the link's own directory, and the broken-link
sweep deletes what it believes is broken. `linkTarget` resolves against
`filepath.Dir(link)`, and every staleness check goes through it.

Where symlinks are not permitted — an unprivileged Windows session — creation
degrades to a directory junction, and then to a copy. Only a permission error
triggers the fallback; anything else surfaces, because silently copying would
hide a real bug. A copy is marked with a sentinel file so a later run can tell
its own copy from a directory the user created, and refresh the former while
refusing to touch the latter.

## What the installer preserves

An install copies a tree, and what it copies faithfully is part of the contract.
Permission bits carry over, because a skill that ships a helper script and a
`SKILL.md` telling the agent to run it is broken by an install that drops the
executable bit — and broken in the worst way, where the tree looks complete and
fails only at the moment of use. Symlinks inside a skill are recreated as
symlinks; one pointing outside it aborts the install rather than copying
whatever it resolves to.

Ownership is not copied. Setting it needs privileges a dependency manager should
never ask for, so files belong to whoever ran `skmgr`. The consequence is that
an agent running as a different user reads them through the group and other
bits, which is why the installer does not narrow modes to owner-only.

## Testing strategy

Three layers, each aimed at a failure the others cannot see.

Unit and command tests cover behavior and are where most regressions get caught.
The ones worth knowing about assert *negative* properties — that a dry run wrote
nothing, that an unmanaged directory was left alone, that a failed install left
no staging directory behind — because this tool's worst bugs delete or overwrite
things rather than return errors.

Fuzzing covers the two parsers that read untrusted bytes. A manifest usually
comes from a repository somebody else controls, so `FuzzParseBytes` asserts that
anything accepted is *safe to act on*, not merely that parsing did not panic: a
name that would escape the store fails the fuzzer even when no error was
returned. `FuzzReadWriteRoundTrip` targets the lockfile's quieter failure, where
a file reads back with different values yet still parses, corrupting a pinned
install without ever erroring. Alongside them sit fixed tests for the inputs a
random mutator will not find — alias bombs, deep nesting, very large scalars.

End-to-end runs in CI against a pinned public commit, because the local fixtures
cannot tell you whether a real repository's layout, tags and content hash behave
as expected. Pinning the commit is what makes it a test rather than a status
check on someone else's branch.

Benchmarks exist to make regressions visible in the paths that run on every
command: directory hashing, discovery walks, lockfile I/O and tag lookup. They
run nightly with a guard rather than asserting absolute numbers, which on shared
runners are noise.

## Tech stack

- Go 1.22+, Cobra, yaml.v3
- System `git` via `os/exec` (no go-git)
- Symlinks via `os.Symlink`, with `mklink /J` junction and copy fallbacks on Windows
- Logging via `internal/log` to stderr, so stdout stays machine-readable
- Advisory file locks via `golang.org/x/sys` (`flock` on Unix, `LockFileEx` on Windows)
- Semver ranges via `github.com/Masterminds/semver/v3`
- JSON Schema validation in tests via `github.com/xeipuuv/gojsonschema`

<!-- last-phase: 5 -->

## Boundaries

```
User / CI
    │  CLI
    ▼
cmd ──► engine ──► provider | installer | linker
              │
              ├── reads/writes skmgr.yml, skmgr.lock
              ├── reads/writes ~/.skmgr/cache and ~/.skmgr/global.lock
              └── reads/writes .agents/ + agent dirs
```

External dependency at runtime: **git on PATH** for remote sources.
