# Proposal

## Why

Nightly has failed on every run since at least 2026-11-03, and the latest run ([43859214588](https://github.com/AAG1999/skmgr/actions/runs/43859214588)) fails in one job: `fuzz (./internal/lockfile, FuzzReadWriteRoundTrip)`. The fuzzer found a lockfile YAML document the reader accepts whose `ref` changes when skmgr writes the file back. That is the quiet corruption the fuzzer exists to catch: a later install can repin a skill without reporting an error. CI, Integration, and Security on `main` are green; this is the failure that is still red.

## What Changes

- Make `skmgr.lock` string fields survive a read → write → read cycle, including YAML block scalars whose content has a leading newline.
- Keep ordinary lockfiles in the same plain-scalar shape CI already greps (`resolved_ref: v1.1.0`, `commit_sha: <sha>`, `version: "2"`).
- If no scalar style can preserve a field, `Write` must fail and leave the existing file untouched instead of persisting a different value.
- Add the failing input as a regression test so the bug does not depend on a 10-minute nightly fuzz run to show up again.
- Add the seed to `FuzzReadWriteRoundTrip` so the next nightly run starts from the known case.

Out of scope: the 2026-11-06 Dependabot CI `lint` failure on the `pflag` bump (no open pull request, and `main` CI is green), benchmark budgets, and the other nightly fuzz targets (`FuzzRead`, `FuzzParseBytes`, `FuzzDeriveName`), which passed.

## Capabilities

### New Capabilities

- `lockfile`: Reading and writing `skmgr.lock`, including the rule that a file the reader accepts must not change entry values when written back.

### Modified Capabilities

- None. The repo has no specs yet.

## Impact

- `internal/lockfile` write path (`Write` in `lockfile.go`) and its tests (`lockfile_test.go`, `fuzz_test.go`).
- On-disk `skmgr.lock` text for values the default YAML encoder already round-trips stays the same. Values it would alter (the failing case is `ref` from the folded scalar `>\n\n   0`, read as a leading newline plus `0`, written back as `0`) are quoted so the original string is kept.
- No dependency or CLI changes. `gopkg.in/yaml.v3` stays at v3.0.1.
