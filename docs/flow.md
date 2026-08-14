# Flow

User and application flows for **skmgr**.

## Command map

| Command | What the user wants | What the app does |
|---------|---------------------|-------------------|
| `init` | Start a project | Detect agents from existing dirs; write `skmgr.yml`; create `.agents/{skills,rules}` |
| `add <source>` | Add a dependency | Sync with the entry appended → write manifest and lock on success |
| `remove <name>` | Drop a dependency | Sync without the entry → unlink, delete canonical copy, scrub merged block |
| `install` | Install from manifest | Sync → write lock. `--frozen` pins to the lock; `--offline` forbids network |
| `update [name...]` | Refresh refs | Sync with the named entries unpinned, the rest held at their locked SHA |
| `sync` | Align disk with manifest | Same pipeline as install |
| `list` | See installed skills | Read manifest and lock; hash what is on disk; print a table or JSON |
| `verify` | Confirm nothing drifted | Hash each installed skill and compare to the lock; never touches the network |
| `outdated` | See what is available | Read each source's tags; report installed, newest in range, newest overall |
| `cache path\|info\|prune` | Inspect or shrink the cache | Report cached commit trees; delete them by age or total size |
| `clean` | Wipe installs / cache | Strip merged blocks, remove `.agents/`, prune broken links; `--global` and `--cache` extend the scope |
| `tree` | See the layout | Group resolved skills by scope, then by agent, and print the shape |
| `doctor` | Find out why something is broken | Check git, config, cache, store, symlink support, and install drift; exit `3` on any failure |
| `self-update` | Get the newest skmgr | Read the latest GitHub release, download the build for this platform, swap the binary atomically. Bootstrap installs use `scripts/install.sh` (Unix) or `scripts/install.ps1` (Windows). |
| `completion <shell>` | Tab completion | Emit a completion script for bash, zsh, fish, or powershell |

Every command that changes the project — `add`, `remove`, `install`, `update`,
`sync` — also takes `--dry-run`, which resolves normally and reports what would
change without writing anything.

## Happy path (new project)

```mermaid
flowchart TD
  A[skmgr init] --> B[Edit skmgr.yml targets]
  B --> C[skmgr add source ...]
  C --> D[Sync]
  D --> E[skmgr list]
  E --> F[Commit skmgr.yml + skmgr.lock]
```

Typical CI path after that: `skmgr install --frozen`.

## Core Sync pipeline

Shared by `install`, `sync`, `update`, and post-`add` / `remove`.

```mermaid
flowchart TD
  A["Load skmgr.yml or skmgr.yaml"] --> B{"frozen and no lock?"}
  B -->|yes| X[Error]
  B -->|no| B2["Build pin map from the lock, keyed by source and path"]
  B2 --> C[Resolve each skill]
  C --> D["Provider fetch: git or local"]
  D --> D2["Diff old lock vs resolved set"]
  D2 --> D3["Unlink dropped names, scrubs merged rule blocks"]
  D3 --> E[For each resolved skill]
  E --> F["Unlink this name from every known agent"]
  F --> F2["Remove the same name from the other type's tree"]
  F2 --> G["Copy into .agents/ for this scope"]
  G --> H[Hash directory]
  H --> I[LinkSkill or LinkRule per target]
  I --> J[Record lock entry]
  J --> K{"more skills?"}
  K -->|yes| E
  K -->|no| L["Clean project orphans in .agents/"]
  L --> L2["Update ~/.skmgr/global.lock for this project"]
  L2 --> L3["Clean global orphans against ledger union"]
  L3 --> M[Prune broken agent symlinks]
  M --> N[Return new lockfile]
```

Two cleanup passes are deliberately different. Project orphans are decided from
this project's manifest alone. Global orphans are decided from the ledger union,
because `~/.agents/` is shared by every project on the machine.

Unlinking sweeps every known agent rather than the current target list, because
a skill whose `targets` shrank still has links under the agents it dropped. The
correct links are recreated in the same iteration, so the only visible effect is
that stale ones disappear. The same reasoning covers a changed `type`: the copy
under the other tree is removed before installing, since orphan cleanup matches
on name and would otherwise consider it valid.

## Global scope and the ledger

```mermaid
flowchart TD
  S["Sync computes this project's global skill names"] --> R["Read ~/.skmgr/global.lock"]
  R --> N{"ledger existed?"}
  N -->|"no, first run after upgrade"| AD["Adopt global skills already on disk"]
  N -->|yes| SET
  AD --> SET["Record project to names mapping"]
  SET --> W["Write ledger"]
  W --> U["valid = union of all projects plus adopted"]
  U --> P["Prune ~/.agents entries not in valid"]
```

Adoption exists so that upgrading from a version without a ledger cannot delete
global skills that skmgr has no ownership record for.

## Resolve detail

```mermaid
flowchart LR
  M[SkillDependency] --> F{source type}
  F -->|git URL| G[GitProvider]
  F -->|"file:// or path"| L[LocalProvider]
  G --> K{"offline?"}
  K -->|"yes, nothing cached"| KE["Error: not in the cache"]
  K -->|"yes, cached"| RG
  K -->|no| FE["clone or fetch, with retries"]
  FE --> RG{"ref is a semver range?"}
  RG -->|yes| TAGS["list the clone's tags, pick the best match"]
  RG -->|no| RS
  TAGS --> RS["resolve ref to a commit SHA"]
  RS --> CH{"commits/sha already extracted?"}
  CH -->|yes| C["reuse the immutable tree"]
  CH -->|no| EX["git archive into staging, then rename"]
  EX --> C
  C --> R["discover path or wildcard"]
  L --> R
  R --> SAFE{"path stays inside source root?"}
  SAFE -->|no| ERR["Error: path escapes the source root"]
  SAFE -->|yes| O["ResolvedSkill + CommitSHA"]
```

- Git: clone or fetch, resolve `ref` to a SHA, extract that commit once, discover
  `path` / wildcards. Clone and fetch run under a per-source lock; the extracted
  tree is immutable, so reads are lock-free and concurrent runs cannot collide.
- Network failures that look transient are retried three times with exponential
  backoff before the command gives up.
- Type auto-detect (wildcard): `SKILL.md` → skill, `AGENTS.md` → rule.
- Discovery expects skills under a parent named `skills`, rules under `rules`.
- A literal `path` is resolved and rejected if it escapes the source root; glob
  matches are re-filtered after expansion for the same reason.

### Choosing the checkout

```mermaid
flowchart TD
  A[requested ref] --> B{offline or pinned to a SHA?}
  B -->|yes| P["use cache as-is, no network"]
  B -->|no| C{ref kind}
  C -->|"empty"| D["ask remote for its default branch"]
  C -->|branch| E["fetch, then check out origin/branch"]
  C -->|tag or SHA| F{already in cache?}
  D --> E
  F -->|yes| P
  F -->|no| G["fetch, then check out"]
```

A branch is a moving target, so the cached clone must follow the remote —
checking out the local branch is what used to serve stale commits forever. A tag
or SHA is immutable, so once cached it is reused and the network is skipped.

## Version ranges

```mermaid
flowchart TD
  A["ref from the manifest"] --> B{"a range, or 'latest'?"}
  B -->|"no: main, v1.2.3, a SHA"| L["use it literally"]
  B -->|yes| T["read the clone's tags"]
  T --> P["keep the ones that parse as semver"]
  P --> N{"any left?"}
  N -->|no| E1["Error: no version tags; use a branch or SHA"]
  N -->|yes| D["drop prereleases unless the range names one"]
  D --> M{"any satisfy the range?"}
  M -->|no| E2["Error: nothing satisfies it, here is what exists"]
  M -->|yes| S["take the newest match"]
  S --> R["record it as resolved_ref, treat it as an immutable tag"]
```

Detection is deliberately narrow: `ref` may also be a branch name, so only
`latest`, a leading operator, a version wildcard, or a compound constraint is
read as a range. A bare `v1.2.3` is an exact tag, and `release-2` is a branch.

Tags come from the cached clone, which is fetched with `--tags` on every run, so
resolution needs no extra network call and works under `--offline`.

`skmgr update` keeps a range: it re-resolves and lands on the newest tag inside
it. `--latest` ignores the range for that one run without rewriting the
manifest, so the next plain update returns to the declared bounds.

## Verify flow

```mermaid
flowchart TD
  A["skmgr verify"] --> B{"skmgr.lock exists?"}
  B -->|no| E["Error: nothing to verify against"]
  B -->|yes| C[For each lock entry]
  C --> D{"found under .agents or ~/.agents?"}
  D -->|no| M["missing"]
  D -->|yes| H["Hash the directory"]
  H --> K{"matches the locked hash?"}
  K -->|no| MOD["modified"]
  K -->|yes| OK["ok"]
  M --> N{"more entries?"}
  MOD --> N
  OK --> N
  N -->|yes| C
  N -->|no| R{"any missing or modified?"}
  R -->|yes| X["Report the table, exit 3"]
  R -->|no| Z["Report the table, exit 0"]
```

Verify never contacts a remote, so it is safe in an air-gapped job and cheap
enough to run as a pre-commit or CI guard. `list` reports the same three states
in its status column, computed the same way.

Exit `3` means drift specifically, as opposed to `1` for a failure to run at
all. That lets a pipeline fail a pull request because a skill was edited without
also failing when the network is down.

## Dry-run flow

```mermaid
flowchart TD
  A["install / update / sync / add / remove --dry-run"] --> B[Load manifest and lock]
  B --> C["Build the same resolver options the real run would use"]
  C --> D["Resolve sources (writes only to the cache)"]
  D --> E[Verify what is installed against the lock]
  E --> F["For each resolved skill"]
  F --> G{"in the lock?"}
  G -->|no| I1["install"]
  G -->|yes| H{"resolved commit differs?"}
  H -->|yes| U["update"]
  H -->|no| V{"on-disk hash matches?"}
  V -->|yes| N["unchanged"]
  V -->|no| RI["reinstall"]
  I1 --> W["Locked but no longer declared: remove"]
  U --> W
  N --> W
  RI --> W
  W --> P["Print the changes; write nothing"]
```

A dry run resolves for real, because there is no way to know whether a branch
has moved without asking. That touches the download cache and nothing else: no
lockfile, no manifest edit, no files installed or deleted. The resolver options
are built by the same code the real run uses, so the plan cannot be computed
under different rules than the run it predicts.

## Doctor flow

```mermaid
flowchart TD
  A["skmgr doctor"] --> B["git on PATH and runnable?"]
  B --> C["config file parses?"]
  C --> D["cache readable and writable?"]
  D --> E{"manifest found?"}
  E -->|no| W["warn: run skmgr init"]
  E -->|yes| F[".agents/ writable?"]
  F --> G["can a symlink be created here?"]
  G --> H{"lockfile exists?"}
  H -->|no| I["warn: run skmgr install"]
  H -->|yes| J["Hash every install against the lock"]
  W --> R{"any check failed?"}
  I --> R
  J --> R
  R -->|yes| X["Print the report, exit 3"]
  R -->|no| Z["Print the report, exit 0"]
```

Checks never stop at the first failure: someone running doctor wants the whole
picture, not one problem at a time. A missing manifest or lockfile is a warning
rather than a failure, because a fresh project is incomplete, not broken. The
symlink check actually creates one, since that is the only way to find out
whether an unprivileged Windows session or an unusual filesystem will allow it.

## Tree flow

```mermaid
flowchart TD
  A["skmgr tree"] --> B[Load manifest and lock]
  B --> C[Hash installs to get each status]
  C --> D[Group by scope, project first]
  D --> E[Sort skills by name within each scope]
  E --> F[Print scope → skill → agent targets]
```

Where `list` is a flat table for scanning, `tree` shows the shape, which is the
quicker way to notice that a skill is missing from an agent you expected it in.

## Install integrity

```mermaid
flowchart TD
  A[Resolved skill] --> B["Copy the tree into .agents/.staging/name"]
  B --> C{"copy succeeded?"}
  C -->|no| D["Discard staging; the previous install is untouched"]
  C -->|yes| E[Hash the staged tree]
  E --> F{"--frozen?"}
  F -->|no| I
  F -->|yes| G{"hash matches the lock?"}
  G -->|no| H["Error: content drifted; previous install kept"]
  G -->|yes| I["Rename staging over the destination"]
  I --> J[Link into agent dirs]
```

The hash is checked before the rename, not after. Detecting drift after the swap
would mean reporting a failure the tool had already committed to disk.

## Link detail

```mermaid
flowchart TD
  S{type} -->|skill| A{"agent reads .agents natively?"}
  A -->|"yes, e.g. Gemini project"| Skip[No skill symlink]
  A -->|no| Sym["Symlink agent skills dir to .agents/skills/name"]
  S -->|rule| R{RuleStrategy}
  R -->|symlink| RS[Symlink into agent rules dir]
  R -->|merge| RMC{"delimiters balanced?"}
  RMC -->|no| RME["Error: refuse to rewrite, name the file"]
  RMC -->|yes| RM[Inject into single file with skmgr delimiters]
  R -->|skip| RK[No rule action]
```

Scope picks the root for the agent directories too: project scope links into
`<project>/.cursor/`, global scope links into `~/.cursor/`.

Linker also maintains a managed block in `.gitignore` for generated agent paths.
Both the merge and the `.gitignore` block refuse to rewrite a file whose managed
delimiters are unbalanced, since doing so would drop everything after the
orphaned marker.

## Add flow

```mermaid
sequenceDiagram
  actor U as User
  participant CLI as cmd/add
  participant Man as manifest
  participant Eng as engine.Sync
  participant Lock as skmgr.lock

  U->>CLI: skmgr add source with flags
  CLI->>CLI: validate type and scope, derive name
  CLI->>Man: parse skmgr.yml or skmgr.yaml
  CLI->>Eng: Sync with the skill appended in memory
  Eng-->>CLI: new lock
  CLI->>Man: write manifest
  CLI->>Lock: write skmgr.lock
```

The manifest is written only after the install succeeds, so a failed add does
not leave a manifest claiming a skill that was never installed. `--name` is
optional: without it the name is derived from the path, or from the repository
name when there is no path.

## Remove flow

```mermaid
sequenceDiagram
  actor U as User
  participant CLI as cmd/remove
  participant Man as manifest
  participant Eng as engine.Sync

  U->>CLI: skmgr remove name
  CLI->>Man: drop skill, write yml
  CLI->>Eng: Sync
  Note over Eng: Sync unlinks names missing from the new manifest, removes orphan dirs, prunes broken links
```

Because the dropped skill is gone from the manifest, Sync sweeps every known
agent for it. That is what removes a merged rule block, which leaves no symlink
for the broken-link pass to catch.

## Install / update / sync

All three run `engine.Sync`; what differs is which entries are allowed to move.

```mermaid
flowchart LR
  I["install"] --> N["nothing pinned: resolve everything"]
  IF["install --frozen"] --> AP["pin every entry to its locked SHA, offline"]
  U["update"] --> N
  UN["update name..."] --> SP["pin everything except the named entries"]
  Y["sync"] --> N
  N --> S[engine.Sync]
  AP --> S
  SP --> S
  S --> W[Write skmgr.lock]
```

| Command | Resolves | Network | Notes |
|---------|----------|---------|-------|
| `install` | Everything | Yes | The default. Moving refs advance. |
| `install --frozen` | Nothing | No | Every entry pinned to its locked SHA. Fails if the lockfile is missing. |
| `update` | Everything | Yes | Same as install; exists to state the intent. Ranges stay inside their bounds. |
| `update <name>...` | Named only | Yes | Others held at their locked SHA. Unknown names are an error. |
| `update --latest` | Everything moving | Yes | Ranges are ignored in favor of the newest tag. The manifest is not rewritten. |
| `sync` | Everything | Yes | Re-align `.agents/` and agent dirs with what is on disk. |
| any, with `--offline` | Cached only | No | Anything not already cached fails. `SKMGR_OFFLINE=1` sets this for a whole job. |
| any, with `--dry-run` | Everything | Yes | Reports the plan and writes nothing — no lockfile, no manifest edit, no installs. |

Even a named update resolves the full set, because an entry missing from the
resolved set looks like an orphan and would be deleted. Pinning, not exclusion,
is what limits the blast radius. Pins are keyed on source and path rather than
name, so wildcard expansions — whose names are generated during resolution — can
be pinned too.

Entries without a recorded commit SHA, which is every local `file://` source,
cannot be pinned and are re-read from disk even under `--frozen`.

## Data flow (runtime dirs)

```
Remote git / local path
        │
        ▼
 ~/.skmgr/cache/git/<sha256(source)>/commits/<commit-sha>/
        │  copy via .agents/.staging, then rename
        ▼
 project scope: .agents/skills/<name>/   or   .agents/rules/<name>/
 global  scope: ~/.agents/skills/<name>/ or   ~/.agents/rules/<name>/
        │  symlink or merge (same root as the scope above)
        ▼
 <root>/.cursor/skills|rules
 <root>/.claude/skills + CLAUDE.md
 <root>/.gemini/...
 <root>/.github/skills + copilot-instructions.md

 where <root> is the project dir for project scope, ~ for global scope
```

Ownership of the global tree is tracked separately in `~/.skmgr/global.lock`.

## Clean flow

```mermaid
flowchart TD
  C[skmgr clean] --> F{flags}
  F -->|default| P[project scope]
  F -->|--global| G[global scope]
  F -->|--cache| K["wipe ~/.skmgr/cache/"]
  F -->|--all| ALL[project + global + cache]
  P --> STRIP
  G --> STRIP
  STRIP["strip every skmgr block from merged rule files"] --> RM["remove the canonical .agents tree for that scope"]
  RM --> SWEEP[prune broken symlinks in agent dirs]
  G --> LED["release this project's claims in the global ledger"]
```

Stripping happens before the canonical tree is deleted: a merged block leaves no
symlink, so once the tree is gone there is nothing left to tell the linker which
blocks to remove. A project-scoped clean never touches the global tree.

## Actors

| Actor | Interaction |
|-------|-------------|
| Developer | Local CLI against project + home dirs |
| Team / VCS | Shares `skmgr.yml` + `skmgr.lock` |
| CI | Runs `skmgr install --frozen`, then `skmgr verify` |
| Agents | Read linked/merged files; not aware of skmgr |

## Recommended CI flow

```mermaid
flowchart TD
  START([job starts]) --> CACHE["restore ~/.skmgr/cache"]
  CACHE --> INSTALL["skmgr install --frozen"]
  INSTALL --> CODE{exit code}
  CODE -->|"0"| VERIFY["skmgr verify"]
  CODE -->|"3"| DRIFT["lockfile and tree disagree: fail loudly"]
  CODE -->|"1 or 2"| BROKEN["real failure: fetch, config or usage"]
  VERIFY --> VCODE{exit code}
  VCODE -->|"0"| RUN([run the agent])
  VCODE -->|"3"| DRIFT
  DRIFT --> DOCTOR["skmgr doctor --json for the full picture"]
  BROKEN --> DOCTOR
```

Branching on `3` separately from `1` is what makes the failure legible: drift
means the committed lockfile no longer describes the tree, which a developer
fixes by rerunning `install` and committing, while `1` means the run itself
could not complete. Treating both as "failed" sends people looking in the wrong
place.

<!-- last-phase: 5 -->
