# Manifest & Lockfile

## Manifest (`skmgr.yml`)

`skmgr.yml` is the source of truth for agent dependencies. Both `skmgr.yml` and `skmgr.yaml` are accepted; `skmgr` searches for `skmgr.yml` first. Keeping both in one project is rejected by `skmgr init`.

```yaml
version: "1"
name: my-agent-workspace

targets:
  - cursor
  - gemini

skills:
  # Name derived from the last path segment -> "skill-creator"
  - source: https://github.com/anthropics/skills.git
    path: skills/skill-creator
    ref: main

  # Same skill under a name you choose
  - name: my-creator
    source: https://github.com/anthropics/skills.git
    path: skills/skill-creator
    ref: main

  # Follow releases with a semver range
  - source: https://github.com/anthropics/skills.git
    path: skills/pdf-tools
    ref: ^1.2.0

  # Auto-discover every skill and rule in a directory
  - source: https://github.com/anthropics/skills.git
    path: "skills/*"
    ref: main

  # No path: name derived from the repo -> "skills"
  - source: https://github.com/mattpocock/skills.git
    ref: main

  # Official guidelines as a global rule
  - source: https://github.com/golang/style.git
    type: rule
    scope: global

  # Local skill with overridden targets
  - source: file:///Users/me/local-skills/deployment
    targets: [cursor]
```

### Fields

- `version` (string): Manifest format version. Currently `"1"`.
- `name` (string): Workspace identifier.
- `targets` (list): Default agents to link to (`cursor`, `gemini`, `claude-code`, `copilot`).
- `skills` (list): Dependencies.
  - `source` (**required**): Git URL or local `file://` path.
  - `name` (optional): Local alias. See [name derivation](#name-derivation).
  - `path` (optional): Subdirectory within the source. Must stay inside the source; `../` escapes are rejected. Supports wildcards (e.g. `skills/*`).
  - `ref` (optional): Git branch, tag, commit SHA, or semver range such as `^1.2.0`. Defaults to the remote's default branch.
  - `type` (optional): `skill` or `rule`. Defaults to `skill`. With wildcards, detected per match from `SKILL.md` or `AGENTS.md`.
  - `scope` (optional): `project` (`.agents/`) or `global` (`~/.agents/`). Defaults to `project`.
  - `targets` (optional): Overrides the manifest-level targets for this skill.

### Name derivation

`name` is optional. When omitted, `skmgr` derives one at parse time:

| Case | Derived from | Example → name |
|------|--------------|----------------|
| `path` is set | its last segment | `path: skills/skill-creator` → `skill-creator` |
| `path` is empty | the repository name | `https://github.com/mattpocock/skills.git` → `skills` |
| `path` is a wildcard | each matched folder, at resolve time | `skills/*` → one entry per folder |

Set `name` explicitly for a different alias, or when two entries would derive the same name. Duplicate names are rejected.

A name becomes a directory name, so it must be a single path component. Names containing `/`, `\`, `..`, characters illegal in a directory name, or Windows reserved device names (`CON`, `NUL`, `COM1`, …) are rejected at parse time.

### How `path` is interpreted

| `path` | Behavior |
|--------|----------|
| A directory, e.g. `skills/pdf-tools` | Installed as written. |
| A wildcard, e.g. `packages/*/agent` | Every match that contains a `SKILL.md` or `AGENTS.md` is installed. |
| Omitted | The repository is searched for directories under a `skills/` or `rules/` parent. |

The `skills/` / `rules/` convention only constrains the third case. A path you write yourself is followed as given.

If a repository has neither layout, `skmgr` installs its root — but only when that root holds a `SKILL.md` or `AGENTS.md`. Without one, the install is refused.

### Validation errors

Manifest problems are reported against the line they are on, with a suggestion where there is an obvious one:

```
Error: failed to read skmgr.yml: manifest validation: line 10: skills[1].targets
has unknown agent "curser" (known: cursor, gemini, claude-code, copilot); did
you mean "cursor"?
```

Unknown keys are rejected rather than ignored.

### Editor autocomplete

A JSON Schema ships at [`schemas/skmgr.schema.json`](../../schemas/skmgr.schema.json):

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/AAG1999/skmgr/main/schemas/skmgr.schema.json
name: my-project
```

---

## Versioning

There is no skill registry, so a git tag is the closest thing to a published version. `ref` accepts a semver range as well as a literal ref:

```yaml
skills:
  - source: https://github.com/anthropics/skills.git
    path: skills/pdf-tools
    ref: ^1.2.0        # newest 1.x, never 2.0
```

| `ref` | Meaning |
|-------|---------|
| `^1.2.0` | Newest release that keeps the same major: `>=1.2.0 <2.0.0` |
| `~1.2.0` | Newest patch of that minor: `>=1.2.0 <1.3.0` |
| `>=1.2.0 <2.0.0` | An explicit window |
| `1.x`, `1.2.*` | Wildcard in a version position |
| `1.0.0 - 1.4.0` | An inclusive span |
| `1.x \|\| 2.x` | Either of two ranges |
| `latest`, `*` | The newest release |
| `v1.2.3`, `main`, a commit SHA | A literal ref |

A plain version such as `v1.2.3` is **not** a range: it is an exact tag.

**Prereleases** are excluded unless you ask for one. `^1.0.0` will not select `v1.3.0-beta.1`, but `>=1.3.0-beta.1` will. Non-semver tags (`nightly`, `stable`) are ignored during range resolution and can still be used as literal refs.

Which tag a range resolved to is recorded in the lockfile:

```yaml
  - name: pdf-tools
    ref: ^1.2.0
    resolved_ref: v1.4.2
    commit_sha: 4f2c1ab9e0d3...
```

### Seeing and taking new versions

```console
$ skmgr outdated
NAME        RANGE    CURRENT   WANTED   LATEST   NOTE
pdf-tools   ^1.2.0   v1.4.2    v1.5.0   v2.1.0
```

`WANTED` is what a plain `skmgr update` would install. `LATEST` is the newest that exists. To cross a major deliberately:

```bash
skmgr update --latest
```

`--latest` is a one-shot override: the manifest is not rewritten. Edit `ref` in `skmgr.yml` when you want the new major to stick.

### Versioning a local repository

A path on disk is read as a plain directory, so `ref` has nothing to act on. Prefix with `git::` when pointing at a git clone:

```yaml
  - source: git::/srv/mirrors/skills.git
    path: skills/pdf-tools
    ref: ^1.2.0
```

---

## Lockfile (`skmgr.lock`)

Running `skmgr install`, `skmgr update`, or `skmgr sync` writes a `skmgr.lock` file. Commit it to version control.

```yaml
version: "2"
generated_at: "2026-08-11T10:04:22Z"
entries:
  - name: skill-creator
    source: https://github.com/anthropics/skills.git
    path: skills/skill-creator
    ref: ^1.2.0
    resolved_ref: v1.4.2
    commit_sha: 4f2c1ab9e0d3...
    content_hash: sha256:9b1e...
    resolved_at: "2026-08-11T10:04:22Z"
```

`resolved_ref` appears only when `ref` was a version range. Older lockfiles (version `1`, or none) are read as-is and upgraded on the next write.

**Frozen installs.** `skmgr install --frozen` pins every skill to the commit SHA in the lockfile. Entries without a recorded SHA (local `file://` sources) are still re-read from disk. Under `--frozen`, fetched content is hashed and compared to the lockfile *before* anything is swapped into place.

**Offline installs.** `--offline` forbids network access. Setting `SKMGR_OFFLINE=1` does the same for every command:

```bash
export SKMGR_OFFLINE=1
skmgr install --frozen
```

**Verifying an install.** `skmgr verify` re-hashes everything on disk and compares it to the lockfile, without touching the network. It exits non-zero when anything is missing or modified.
