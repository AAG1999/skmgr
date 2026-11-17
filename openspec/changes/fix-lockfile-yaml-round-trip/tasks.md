# Tasks

## 1. Round-trip encoding in Write

- [x] 1.1 In `internal/lockfile.Write`, encode with the current `yaml.Encoder` and compare the unmarshaled entry strings, `version`, and `generated_at` to the in-memory lock. When they match, keep those bytes. When they do not, re-encode as a `yaml.Node` document that double-quotes only the string scalars whose default style does not unmarshal to the same string, then compare again. Verify with `go test ./internal/lockfile/ -count=1 -run 'TestWrite_PreservesLeadingNewlineInRef|TestWriteAndRead|TestRead_'`, including a new test that reads `entries:\n- ref: >\n\n   0`, writes it, and reads back `ref` as a newline followed by `0`, and a case whose optional strings are empty and stay empty.
- [x] 1.2 Add a unit test that writes an entry with resolved ref `v1.1.0` and a hex commit SHA and asserts the file contains `resolved_ref: v1.1.0`, a plain `commit_sha` scalar, and `version: "2"`. Verify with `go test ./internal/lockfile/ -count=1 -run TestWrite_KeepsPlainPins`.
- [x] 1.3 Return the encode error before `atomicfile.Write` when the second document still differs. Verify with a same-package test that an existing destination keeps its previous bytes and a missing destination is not created when that mismatch is reported.
- [x] 1.4 Add the Nightly input `entries:\n- ref: >\n\n   0` to `lockSeeds` in `internal/lockfile/fuzz_test.go`. Verify with `go test ./internal/lockfile/ -count=1 -run 'FuzzReadWriteRoundTrip|FuzzRead'`.

## 2. Package check

- [x] 2.1 Run `go test ./internal/lockfile/ -count=1` and confirm the package passes, including the existing write, version, and hash tests.

## 3. CI lint pin

- [ ] 3.1 In `.github/workflows/ci.yml`, change the lint job's `golangci-lint-action` `version` from `v2.12` to `v2.15` and rewrite the comment above it so it names the Go 1.27 toolchain instead of 1.26. Verify by running the CI `lint` job on the pushed branch and confirming the `golangci-lint` step exits 0 without the `file requires newer Go version` panic. If the step reports findings, report them with file and line instead of adding exclusions.

## Workflow follow-up

- Archive the change after review.
