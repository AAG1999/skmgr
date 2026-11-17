# Design

## Context

See proposal.md for why Nightly is red. `internal/lockfile.Write` sorts entries, stamps `generated_at`, and encodes the struct with `yaml.Encoder` (indent 2) into `atomicfile.Write`. `Read` unmarshals with `yaml.Unmarshal` into `types.Lockfile`.

The failing input is the bytes `entries:\n- ref: >\n\n   0`. Unmarshal yields a single entry with an empty name and `Ref` equal to a newline followed by `0`. The default encoder emits a block scalar for that newline, and the next unmarshal yields `0`. `FuzzReadWriteRoundTrip` compares the two `LockEntry` values and fails. The same encoder is used by `WriteGlobalLedger`; that path is not what Nightly runs.

CI greps written lockfiles for `resolved_ref: v1.1.0`, `commit_sha: <sha>`, and `version: "2"` (`.github/workflows/ci.yml`, `.github/workflows/integration.yml`). Quoting every string would break those checks and rewrite committed lockfiles.

The CI lint job (`.github/workflows/ci.yml`, `golangci/golangci-lint-action@v7`, `version: v2.12`) runs `golangci-lint` v2.12.2, which is built with Go 1.26. The job installs Go `stable`, now 1.27, and the linter panics while loading packages with `file requires newer Go version go1.27 (application built with go1.26)`. The v2.13.0 through v2.15.0 releases are built with Go 1.27.0 (their release workflow sets `GO_VERSION: '1.27.0'`). The existing comment in `ci.yml` names 1.26 as the floor and needs the same update.

## Goals / Non-Goals

**Goals:**

- `Read` → `Write` → `Read` keeps every entry string field, for every document `Read` already accepts.
- Detect a lossy encode before the file is replaced.
- Leave the on-disk shape of ordinary pins unchanged.
- Make the CI lint job load the Go toolchain that `go-version: 'stable'` installs.

**Non-Goals:**

- Rejecting odd refs at read time. A value the reader already accepted must be preserved, not stripped.
- Changing `WriteGlobalLedger`, manifest writing, or the YAML library version.
- Forcing double quotes on values the current encoder already round-trips.
- Changing `.golangci.yml`, the enabled linters, or the `go` directive in `go.mod`.
- Pinning the lint job's Go toolchain back to 1.26. That would keep the lint job on an older toolchain than the test matrix's `stable` leg.

## Decisions

### Probe the encoded document, then quote only the fields that moved

Encode with the existing `yaml.Encoder`. Unmarshal those bytes and compare entry string fields (and `version`, `generated_at`) to the in-memory lock after the sort and timestamp stamp `Write` already does.

If they match, pass those bytes to `atomicfile.Write`. That is the common path, so `version: "2"`, `resolved_ref: v1.1.0`, and plain `commit_sha` stay as they are today.

If any field differs, encode again as a `yaml.Node` document. Each string scalar uses the default style unless a one-field probe shows that style does not unmarshal to the same string; those scalars use `yaml.DoubleQuotedStyle`, which writes a leading newline as `\n`. Unmarshal and compare again. If they still differ, return an error and do not call `atomicfile.Write`, so a missing path stays missing and an existing file keeps its bytes (the temp file `atomicfile` uses is never renamed).

Alternatives considered:

- Trim whitespace in `Read`. The fuzzer would go green while the pin still changed, which is the bug the test describes.
- Double-quote every string. Preserves the failing input but fails the plain-scalar greps and dirties every lockfile.
- Treat the fuzzer as too strict and drop the equality check. That removes the only signal for this class of bug.

### Keep the regression next to the fuzzer, not only in the corpus cache

Add the Nightly input as a seed in `lockSeeds` (`internal/lockfile/fuzz_test.go`) and as a table case in `internal/lockfile/lockfile_test.go` that reads, writes, and reads it back. Nightly restores `~/.cache/go-build/fuzz`, so a seed in the test binary is what a clean run actually starts from. A short local `go test -fuzz` is not a substitute for that seed; the unit test is what CI runs on every pull request.

Also assert, in the unit test, that an entry with `resolved_ref: v1.1.0` and a hex `commit_sha` is written with those plain scalars and `version: "2"`.

### Pin golangci-lint to v2.15 and keep the toolchain on `stable`

Set `version: v2.15` in the lint job's `golangci-lint-action` step, and rewrite the comment above it to name the Go 1.27 requirement instead of 1.26. v2.15.0 is the latest tag and uses the same release toolchain as v2.13 and v2.14, so the job stays on the same pinned minor-version style the workflow already uses.

Alternatives considered:

- Keep v2.12 and set the lint job's `go-version` to `1.26`. Rejected: it hides the drift between the lint job and the test matrix's `stable` leg, and the next toolchain bump would fail the same way.
- Pin to v2.13.x or v2.14.x. Also built with Go 1.27, but v2.15 is the latest and avoids a second bump soon after.

## Risks / Trade-offs

- [Double-quoted style also drops a character] → The second compare runs before `atomicfile.Write`. The write fails closed instead of publishing a different pin. No such input is known for v3.0.1; the known failure is the leading newline, which `\n` in a double-quoted scalar keeps.
- [A future fuzz case is a non-string YAML quirk, such as a timestamp reformatted into `resolved_at`] → The same probe covers every string field, not only `ref`. If neither style preserves it, `Write` errors rather than changing the pin.
- [Quoted unusual values show up in diffs the next time skmgr rewrites that lockfile] → Only fields that would otherwise change are quoted. Normal pins stay plain, so a routine install does not reformat the file.
- [`WriteGlobalLedger` can lose the same bytes] → Out of scope. It is not failing in CI. Reuse the helper later if that writer grows a fuzzer; do not expand this change to it.
- [`golangci-lint` v2.15 reports new findings in existing code] → Because `.golangci.yml` enables a fixed set of linters, a version bump changes analyzer behavior, not the enabled list. If the lint job reports findings, stop and report them with file and line. Do not add exclusions to make the job pass.

## Migration Plan

No format version bump. Existing lockfiles still read. The next write of a normal lockfile stays plain; the next write of a document with a leading newline in a field quotes that field and keeps the value. Rollback is reverting the write-path change; already-quoted unusual fields still parse.

The CI pin rolls back by setting `version: v2.12` again. That restores the panic on `stable` and is only a fallback.

## Open Questions

None.
