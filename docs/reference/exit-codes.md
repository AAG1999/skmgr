# Exit codes

A script should be able to tell "your project is out of date" apart from
"skmgr could not run", so those states have separate codes rather than both
collapsing into `1`.

| Code | Name | Meaning |
| --- | --- | --- |
| `0` | success | The command did what was asked. |
| `1` | error | An ordinary failure: a bad manifest, an unreachable remote, a permission problem. |
| `2` | usage | The command line was wrong: an unknown flag, a missing argument, an unknown subcommand. |
| `3` | drift | The command ran fine and found the project out of line with what was declared. |

## When you get a `3`

Code `3` means skmgr worked correctly and the answer was "no". It is produced
by:

- `skmgr verify` when an installed skill no longer matches its lockfile hash.
- `skmgr doctor` when any check fails.
- `skmgr install --frozen` when a fetched skill's content hash differs from the
  one recorded in `skmgr.lock`.

This is the code to branch on in CI when you want a pull request to fail
because someone edited a vendored skill in place, without also failing when the
network is down.

```bash
skmgr verify
case $? in
  0) echo "everything matches the lockfile" ;;
  3) echo "installed skills have drifted; run skmgr sync"; exit 1 ;;
  *) echo "skmgr could not run"; exit 1 ;;
esac
```

## Gating a pipeline

The common shape is a frozen install followed by a verify:

```bash
skmgr install --frozen || exit 1
skmgr verify           || exit 1
```

Both fail the job, but the codes tell you which happened when you read the log
afterwards.
