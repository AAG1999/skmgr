# CLI Reference

| Command | Synopsis |
|---------|----------|
| `skmgr init` | Creates `skmgr.yml` and `.agents/` on first run, and writes the chat workflow files for the agents you pick. On a terminal it asks; `--targets` skips the question. Running it again updates `targets` and refreshes unedited workflow files. It does not change the skills list or `skmgr.lock`. `--dry-run` writes nothing. `--force` overwrites workflow files you edited. |
| `skmgr add <source>` | Adds a dependency to the manifest and installs it. Use `--name`, `--path`, `--ref`, `--type`, and `--scope` to customize. |
| `skmgr remove <name>` | Removes a skill from the manifest, deletes its canonical copy, unlinks it from every target, and scrubs any merged rule block. |
| `skmgr discover <source>` | Lists the skills and rules one git URL or local path offers, with the path and ref to pass to `add`. `--ref`, `--path`, and `--offline`. Writes only to the download cache. |
| `skmgr clean` | Removes installed skills and rules, strips managed rule blocks, and prunes broken symlinks. Workflow files skmgr generated are left in place. `--cache` wipes the download cache, `--global` cleans `~/.agents/`, `--all` does everything. |
| `skmgr install` | Installs everything in `skmgr.yml`. `--frozen` installs exactly what the lockfile pins; `--offline` forbids network access. |
| `skmgr update [name...]` | Re-resolves refs and regenerates the lockfile. Semver ranges stay inside range; `--latest` takes the newest release regardless. It does not refresh workflow files; run `skmgr init` again for that. |
| `skmgr outdated` | Compares each skill against its repository's tags. `--json` for scripting. |
| `skmgr list` | Table of every skill with type, scope, ref, status, and targets. `--json` for scripting. |
| `skmgr sync` | Same pipeline as `install`. Use after hand-editing the manifest. |
| `skmgr verify` | Checks every installed skill against the lockfile's hashes. No network. Exits non-zero if anything drifted. |
| `skmgr cache path` | Prints the cache directory. |
| `skmgr cache info` | Lists cached commit trees with size and last use. |
| `skmgr cache prune` | Deletes cached trees. `--older-than 30d`, `--max-size 2GB`, `--dry-run`. |
| `skmgr tree` | Shows which skills exist, at which scope, and which agents each is linked into. |
| `skmgr doctor` | Checks git, config, cache, store, symlink support, lockfile match, and workflow files. Exits `3` if any check fails. A workflow file you edited is not a failure. |
| `skmgr self-update` | Downloads the newest release and replaces this binary. `--check` reports without installing. First-time installs use the README one-liners (`install.sh` / `install.ps1`). |
| `skmgr completion <shell>` | Prints a completion script for `bash`, `zsh`, `fish`, or `powershell`. |

## Previewing changes

`init`, `install`, `sync`, `update`, `add`, `remove`, and `clean` take `--json`. `discover` does too, and so does `--dry-run` on the commands that have it. Stdout is one JSON document. Logs stay on stderr. A failure is `{"error":{"code":"...","message":"...","fix":"..."}}` with the same exit status as the human output. `doctor` and `verify` keep printing their report and exit `3`; they do not also print that error document.

`add`, `remove`, `install`, `sync`, and `update` use this shape. `changes` is an array and is empty when there is nothing to do. `action` is `install`, `update`, `reinstall`, or `remove`.

```json
{
  "dry_run": true,
  "changes": [
    {
      "name": "pdf-tools",
      "action": "install",
      "type": "skill",
      "scope": "project",
      "source": "file:///path/to/source",
      "path": "skills/pdf-tools",
      "targets": ["cursor"]
    }
  ]
}
```

`init --json` reports the agents you selected and what it would do to each workflow file (`add`, `change`, `delete`, or `skip`). A cursor selection lists eight files; each entry has this shape:

```json
{
  "targets": ["cursor"],
  "files": [
    {"path": ".cursor/skills/skmgr-find-skill/SKILL.md", "action": "add"}
  ]
}
```

`discover --json` reports the source, the resolved ref and commit when the source is git, and one entry per skill or rule:

```json
{
  "source": "file:///path/to/source",
  "entries": [
    {
      "name": "pdf-tools",
      "type": "skill",
      "path": "skills/pdf-tools",
      "description": "Fills PDF forms"
    }
  ]
}
```

`list`, `tree`, `verify`, `outdated`, and `doctor` keep the JSON fields they already had. Each doctor check is `name`, `state` (`pass`, `warn`, or `fail`), `detail`, and `remedy`. `clean --json` prints `{"cleaned": true}`.

`install`, `sync`, `update`, `add`, `remove`, and `init` all take `--dry-run`. It resolves normally and prints what would change, but writes nothing:

```bash
$ skmgr update --dry-run
ACTION   NAME       CHANGE
update   greeter    v1.2.0 (a1b2c3d) -> v1.3.1 (e4f5g6h)
install  reviewer   new at v0.4.0

2 changes. Run without --dry-run to apply.
```

## Global flags

| Flag | Effect |
|------|--------|
| `--verbose` / `-v` | Lower the log level to `debug`. |
| `--quiet` / `-q` | Raise the log level to `error`. Mutually exclusive with `--verbose`. |
| `--log-level <level>` | Set the level explicitly: `debug`, `info`, `warn`, or `error`. |

Logs go to stderr, so command output stays pipeable.

`--global` / `-g` is **not** a global flag. It is declared only on `add`, `list`, and `clean`. `install`, `sync`, and `update` take each skill's scope from its `scope:` in the manifest and reject `--global`.

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | Success. |
| `1` | Ordinary failure: bad manifest, unreachable remote, permission problem. |
| `2` | Wrong command line: unknown flag, missing argument. |
| `3` | Project out of line with what was declared — failed `verify`, failed `doctor`, or `--frozen` hash mismatch. |

See [exit-codes.md](exit-codes.md) for CI branching guidance.

## Configuration

Machine-level preferences live in `~/.config/skmgr/config.yml` (optional):

```yaml
cache_dir: /mnt/fast/skmgr-cache
log_level: info
targets:
  - cursor
  - claude-code
```

| Variable | Effect |
|----------|--------|
| `SKMGR_CONFIG` | Path to the config file. |
| `SKMGR_HOME` | Where skmgr keeps its state: cache and global ledger. |
| `SKMGR_CACHE_DIR` | The download cache on its own. |
| `SKMGR_OFFLINE` | Forbids network access, like `--offline`. |
| `SKMGR_LOG_LEVEL` | Default verbosity. |

A flag beats an environment variable, which beats the config file, which beats the default. Full details in [config.md](config.md) and [env-vars.md](env-vars.md).
