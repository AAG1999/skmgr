# Environment variables

Every setting below can also be written to the [config file](config.md). A
command-line flag beats an environment variable, which beats the config file,
which beats the built-in default.

| Variable | Default | What it does |
| --- | --- | --- |
| `SKMGR_CONFIG` | `~/.config/skmgr/config.yml` | Path to the config file. Set it to a path that does not exist to run with defaults only. |
| `SKMGR_HOME` | `~/.skmgr` | Where skmgr keeps its own state: the download cache and the global install ledger. |
| `SKMGR_CACHE_DIR` | `$SKMGR_HOME/cache` | Where downloaded repositories are cached. Set this on its own to move the cache without moving the ledger. |
| `SKMGR_OFFLINE` | unset | Forbids all network access when set to anything other than `0`, `false`, `no`, or `off`. |
| `SKMGR_LOG_LEVEL` | `info` | Verbosity: `debug`, `info`, `warn`, or `error`. |

## Notes on each

### `SKMGR_CONFIG`

Points at a single YAML file. A missing file is not an error, so this is the
way to guarantee a run ignores whatever config the machine happens to have:

```bash
SKMGR_CONFIG=/nonexistent skmgr install
```

### `SKMGR_HOME`

Moves the whole state directory. Useful in a container where `$HOME` is not
writable, or when several checkouts should not share a cache.

```bash
SKMGR_HOME=/var/lib/skmgr skmgr install
```

The cache follows automatically unless `SKMGR_CACHE_DIR` is also set.

### `SKMGR_CACHE_DIR`

Moves only the cache. This is the one to set in CI, because the cache is the
part worth restoring between jobs:

```yaml
- uses: actions/cache@v4
  with:
    path: ~/.skmgr/cache
    key: skmgr-${{ hashFiles('skmgr.lock') }}
```

### `SKMGR_OFFLINE`

Equivalent to passing `--offline` to every command. Any value other than the
explicit negatives turns it on, because a variable that is set at all is
normally meant to enable something:

```bash
SKMGR_OFFLINE=1 skmgr install    # offline
SKMGR_OFFLINE=0 skmgr install    # online
```

There is no way to turn the network back on from the command line when this is
set. A machine configured as offline stays offline.

### `SKMGR_LOG_LEVEL`

Sets the default verbosity. `--verbose`, `--quiet`, and `--log-level` all
override it for a single command.

### Path handling

`SKMGR_HOME` and `SKMGR_CACHE_DIR` expand a leading `~` and are resolved to
absolute paths. A relative value is resolved against the directory the command
runs in, which would otherwise give you a different cache per subdirectory.

## Git-related variables

skmgr shells out to `git`, so git's own configuration applies unchanged. The
ones that come up most often:

| Variable | What it does |
| --- | --- |
| `GIT_SSH_COMMAND` | The SSH command used for `git@` sources, including which key to offer. |
| `HTTPS_PROXY`, `HTTP_PROXY`, `NO_PROXY` | Proxy settings for `https://` sources. |
| `GIT_SSL_CAINFO` | A CA bundle for an internal server with a private certificate. |
| `GIT_TERMINAL_PROMPT=0` | Fail instead of prompting for credentials, which is what you want in CI. |

See [private repositories and proxies](private-git.md) for how to put these
together.
