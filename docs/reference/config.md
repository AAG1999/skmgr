# Config file

skmgr reads user-level settings from `~/.config/skmgr/config.yml`. The file is
entirely optional: most people never write one.

It holds preferences about *this machine* — where the cache lives, how loud the
output is. It is not a place for project dependencies, which belong in
`skmgr.yml` so the whole team gets the same ones.

## Location

| Platform | Default path |
| --- | --- |
| Linux | `~/.config/skmgr/config.yml` |
| macOS | `~/Library/Application Support/skmgr/config.yml` |
| Windows | `%AppData%\skmgr\config.yml` |

Set `SKMGR_CONFIG` to read from somewhere else.

The file lives under the XDG config directory rather than in `~/.skmgr`,
because `~/.skmgr` holds generated state that you should be able to delete to
clear a cache without losing something you hand-wrote.

## Example

```yaml
# Where skmgr keeps its own state.
home: ~/.skmgr

# Where downloaded repositories are cached. Defaults to <home>/cache.
cache_dir: /mnt/fast/skmgr-cache

# Forbid network access for every command.
offline: false

# Default verbosity: debug, info, warn, or error.
log_level: info

# Agents to write into a new skmgr.yml when `skmgr init` cannot detect any.
targets:
  - cursor
  - claude-code
```

Every key is optional. An absent key means "no opinion", which lets an
environment variable or the default take over.

## Paths

`home` and `cache_dir` accept a leading `~`, which expands to your home
directory — a config file is hand-written, and a shell would have expanded it
for you.

Both are then resolved to absolute paths. A relative path would otherwise point
somewhere different depending on which directory you ran `skmgr` from, so
`skmgr install` in a project and the same command one level down would use two
different caches. Keeping them absolute is also what lets Go apply the `\\?\`
prefix on Windows, which is how the 260-character path limit is avoided.

## Keys

| Key | Type | Default | What it does |
| --- | --- | --- | --- |
| `home` | string | `~/.skmgr` | The state directory: cache and global install ledger. |
| `cache_dir` | string | `<home>/cache` | The download cache, on its own so it can go on a different disk. |
| `offline` | boolean | `false` | Forbids network access, as though `--offline` were passed everywhere. |
| `log_level` | string | `info` | Default verbosity. |
| `targets` | list of strings | none | Agents `skmgr init` writes into a new manifest when it cannot detect any from the project. |

## Precedence

From most to least specific:

1. A command-line flag (`--offline`, `--log-level`).
2. An environment variable (`SKMGR_OFFLINE`, `SKMGR_LOG_LEVEL`) — see
   [environment variables](env-vars.md).
3. This file.
4. The built-in default.

So a config file saying `offline: true` can be overridden for one command with
`SKMGR_OFFLINE=0`, and `log_level: warn` can be overridden with `--verbose`.

## Errors

A config file that cannot be parsed stops the command rather than being
ignored, because continuing would mean acting on settings the user believes are
in effect. Unknown keys are rejected too, so a typo like `cach_dir` is reported
instead of silently doing nothing.

`skmgr doctor` reports which config file is in effect, or that there is none:

```
[ ok ] config     reading /home/me/.config/skmgr/config.yml
```
