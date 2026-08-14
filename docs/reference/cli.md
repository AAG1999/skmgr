# CLI Reference

| Command | Synopsis |
|---------|----------|
| `skmgr init` | Creates `skmgr.yml` and `.agents/`. Auto-detects target agents. Refuses to overwrite an existing `skmgr.yml` or `skmgr.yaml`. |
| `skmgr add <source>` | Adds a dependency to the manifest and installs it. Use `--name`, `--path`, `--ref`, `--type`, and `--scope` to customize. |
| `skmgr remove <name>` | Removes a skill from the manifest, deletes its canonical copy, unlinks it from every target, and scrubs any merged rule block. |
| `skmgr clean` | Removes `.agents/`, strips managed rule blocks, and prunes broken symlinks. `--cache` wipes the download cache, `--global` cleans `~/.agents/`, `--all` does everything. |
| `skmgr install` | Installs everything in `skmgr.yml`. `--frozen` installs exactly what the lockfile pins; `--offline` forbids network access. |
| `skmgr update [name...]` | Re-resolves refs and regenerates the lockfile. Semver ranges stay inside range; `--latest` takes the newest release regardless. |
| `skmgr outdated` | Compares each skill against its repository's tags. `--json` for scripting. |
| `skmgr list` | Table of every skill with type, scope, ref, status, and targets. `--json` for scripting. |
| `skmgr sync` | Same pipeline as `install`. Use after hand-editing the manifest. |
| `skmgr verify` | Checks every installed skill against the lockfile's hashes. No network. Exits non-zero if anything drifted. |
| `skmgr cache path` | Prints the cache directory. |
| `skmgr cache info` | Lists cached commit trees with size and last use. |
| `skmgr cache prune` | Deletes cached trees. `--older-than 30d`, `--max-size 2GB`, `--dry-run`. |
| `skmgr tree` | Shows which skills exist, at which scope, and which agents each is linked into. |
| `skmgr doctor` | Checks git, config, cache, store, symlink support, and lockfile match. Exits `3` if any check fails. |
| `skmgr self-update` | Downloads the newest release and replaces this binary. `--check` reports without installing. First-time installs use the README one-liners (`install.sh` / `install.ps1`). |
| `skmgr completion <shell>` | Prints a completion script for `bash`, `zsh`, `fish`, or `powershell`. |

## Previewing changes

`install`, `sync`, `update`, `add`, and `remove` all take `--dry-run`. It resolves normally and prints what would change, but writes nothing:

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
