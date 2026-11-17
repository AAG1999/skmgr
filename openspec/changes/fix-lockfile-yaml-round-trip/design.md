# Design

## Context

See proposal.md for why Nightly is red. `internal/lockfile.Write` sorts entries, stamps `generated_at`, and encodes the struct with `yaml.Encoder` (indent 2) into `atomicfile.Write`. `Read` unmarshals with `yaml.Unmarshal` into `types.Lockfile`.

The failing input is the bytes `entries:\n- ref: >\n\n   0`. Unmarshal yields a single entry with an empty name and `Ref` equal to a newline followed by `0`. The default encoder emits a block scalar for that newline, and the next unmarshal yields `0`. `FuzzReadWriteRoundTrip` compares the two `LockEntry` values and fails. The same encoder is used by `WriteGlobalLedger`; that path is not what Nightly runs.

CI greps written lockfiles for `resolved_ref: v1.1.0`, `commit_sha: <sha>`, and `version: "2"` (`.github/workflows/ci.yml`, `.github/workflows/integration.yml`). Quoting every string would break those checks and rewrite committed lockfiles.

## Goals / Non-Goals

**Goals:**

- `Read` → `Write` → `Read` keeps every entry string field, for every document `Read` already accepts.
- Detect a lossy encode before the file is replaced.
- Leave the on-disk shape of ordinary pins unchanged.

**Non-Goals:**

- Rejecting odd refs at read time. A value the reader already accepted must be preserved, not stripped.
- Changing `WriteGlobalLedger`, manifest writing, or the YAML library version.
- Forcing double quotes on values the current encoder already round-trips.

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

## Risks / Trade-offs

- [Double-quoted style also drops a character] → The second compare runs before `atomicfile.Write`. The write fails closed instead of publishing a different pin. No such input is known for v3.0.1; the known failure is the leading newline, which `\n` in a double-quoted scalar keeps.
- [A future fuzz case is a non-string YAML quirk, such as a timestamp reformatted into `resolved_at`] → The same probe covers every string field, not only `ref`. If neither style preserves it, `Write` errors rather than changing the pin.
- [Quoted unusual values show up in diffs the next time skmgr rewrites that lockfile] → Only fields that would otherwise change are quoted. Normal pins stay plain, so a routine install does not reformat the file.
- [`WriteGlobalLedger` can lose the same bytes] → Out of scope. It is not failing in CI. Reuse the helper later if that writer grows a fuzzer; do not expand this change to it.

## Migration Plan

No format version bump. Existing lockfiles still read. The next write of a normal lockfile stays plain; the next write of a document with a leading newline in a field quotes that field and keeps the value. Rollback is reverting the write-path change; already-quoted unusual fields still parse.

## Open Questions

None.
