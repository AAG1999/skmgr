# Spec Delta

## Purpose

The lockfile records the resolved pin of every skill so a later install checks out the same commit and content the file describes.

## ADDED Requirements

### Requirement: Accepted entry values survive a rewrite
A lockfile the reader accepts SHALL be writable. Reading the file that was written SHALL return the same entry string fields the reader returned, including name, source, path, ref, resolved ref, commit SHA, content hash, and resolved-at. A leading newline in one of those fields is part of the value and SHALL be kept.

#### Scenario: Folded ref with a leading newline
- **WHEN** the reader accepts a lockfile whose only entry has an empty name and a `ref` decoded from the folded scalar `>` with a blank line and then `0` (file bytes `entries:\n- ref: >\n\n   0`)
- **THEN** writing that lockfile and reading it again yields one entry whose `ref` is still a newline followed by `0`

#### Scenario: Empty optional fields stay empty
- **WHEN** the reader accepts an entry whose optional string fields are empty
- **THEN** writing that lockfile and reading it again still yields empty strings for those fields

### Requirement: A rewrite that cannot preserve a value does not replace the file
If the writer cannot encode an accepted entry so that every string field reads back unchanged, the write SHALL fail. An existing lockfile at the destination SHALL keep its previous bytes. A destination that did not exist SHALL still not exist.

#### Scenario: Existing file is left untouched
- **WHEN** a lockfile is already on disk and encoding the in-memory entries would change a string field
- **THEN** the write returns an error and the bytes at that path are unchanged

#### Scenario: Missing file is not created
- **WHEN** no lockfile exists at the destination and encoding the in-memory entries would change a string field
- **THEN** the write returns an error and the destination path is still absent

### Requirement: Ordinary pins stay plain scalars
Rewriting a lockfile whose entry strings are single-line and have no leading or trailing whitespace SHALL keep those values as plain YAML scalars. The lockfile version SHALL still be written as `version: "2"`.

#### Scenario: Resolved ref and commit SHA stay unquoted
- **WHEN** an entry has resolved ref `v1.1.0` and a hex commit SHA, with no leading or trailing whitespace on any entry string
- **THEN** the written file contains the line `resolved_ref: v1.1.0`, contains `commit_sha:` followed by that SHA as a plain scalar, and contains `version: "2"`
