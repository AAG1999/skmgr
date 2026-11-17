// Copyright 2026 AAG1999
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lockfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestRead_Valid(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "lockfiles", "valid.lock")
	lock, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if lock == nil {
		t.Fatal("Read() returned nil lockfile")
	}

	// A version 1 file is readable and is upgraded in place, so the next write
	// stamps it as current.
	if lock.Version != types.LockfileVersion {
		t.Errorf("Version = %q, want %q", lock.Version, types.LockfileVersion)
	}
	if len(lock.Entries) != 1 {
		t.Fatalf("Entries length = %d, want 1", len(lock.Entries))
	}

	entry := lock.Entries[0]
	if entry.Name != "test-skill" {
		t.Errorf("Entry Name = %q, want test-skill", entry.Name)
	}
	if entry.CommitSHA != "abc123def456" {
		t.Errorf("Entry CommitSHA = %q", entry.CommitSHA)
	}
}

// A missing lockfile is reported distinctly so callers can tell it apart from
// a lockfile that exists but is damaged.
func TestRead_NotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does_not_exist.lock")

	lock, err := Read(path)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("Read() for a missing file should return ErrNotFound, got: %v", err)
	}
	if lock != nil {
		t.Fatalf("Read() for a missing file should return a nil lockfile, got: %v", lock)
	}
}

// Silently discarding a damaged lockfile would throw away pinned state the
// user probably wants back.
func TestRead_CorruptIsNotTreatedAsMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.lock")

	if err := os.WriteFile(path, []byte("entries: [this is not: valid: yaml"), 0644); err != nil {
		t.Fatal(err)
	}

	lock, err := Read(path)
	if err == nil {
		t.Fatal("expected an error for a corrupt lockfile")
	}
	if errors.Is(err, ErrNotFound) {
		t.Error("a corrupt lockfile must not be reported as missing")
	}
	if lock != nil {
		t.Error("no lockfile should be returned when parsing fails")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("the error should name the file, got: %v", err)
	}
}

// A lockfile from a newer skmgr may carry constraints this binary would ignore.
func TestRead_RejectsUnknownVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.lock")

	contents := "version: \"99\"\ngenerated_at: 2026-01-01T00:00:00Z\nentries: []\n"
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := Read(path)
	if err == nil {
		t.Fatal("expected an error for an unknown lockfile version")
	}
	if !strings.Contains(err.Error(), "99") {
		t.Errorf("the error should mention the version found, got: %v", err)
	}
}

// A version 1 lockfile has to keep working: one team member upgrading skmgr
// must not break everyone still on the older build until they all upgrade.
func TestRead_UpgradesVersionOneInPlace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.lock")

	contents := `version: "1"
generated_at: "2026-01-01T00:00:00Z"
entries:
  - name: legacy
    source: https://example.com/a.git
    path: skills/legacy
    commit_sha: abc123
    content_hash: sha256:abc
    resolved_at: "2026-01-01T00:00:00Z"
`
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	lock, err := Read(path)
	if err != nil {
		t.Fatalf("a version 1 lockfile should still load: %v", err)
	}
	if lock.Version != types.LockfileVersion {
		t.Errorf("Version = %q, want it upgraded to %q", lock.Version, types.LockfileVersion)
	}
	if len(lock.Entries) != 1 || lock.Entries[0].CommitSHA != "abc123" {
		t.Fatalf("the entry should survive the upgrade, got %+v", lock.Entries)
	}

	// The new fields are simply absent, not invalid.
	if lock.Entries[0].Ref != "" || lock.Entries[0].ResolvedRef != "" {
		t.Errorf("expected the version 2 fields to be empty, got %+v", lock.Entries[0])
	}

	// Writing it back stamps the new version without disturbing the entry.
	if err := Write(path, lock); err != nil {
		t.Fatal(err)
	}

	reread, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if reread.Version != types.LockfileVersion {
		t.Errorf("rewritten version = %q, want %q", reread.Version, types.LockfileVersion)
	}
	if reread.Entries[0].CommitSHA != "abc123" {
		t.Errorf("the commit should be unchanged, got %q", reread.Entries[0].CommitSHA)
	}
}

func TestLockEntry_EffectiveRef(t *testing.T) {
	cases := []struct {
		name  string
		entry types.LockEntry
		want  string
	}{
		{
			name:  "a range reports the tag it resolved to",
			entry: types.LockEntry{Ref: "^1.2.0", ResolvedRef: "v1.4.2"},
			want:  "v1.4.2",
		},
		{
			name:  "a literal ref reports itself",
			entry: types.LockEntry{Ref: "main"},
			want:  "main",
		},
		{
			name:  "an entry with neither reports nothing",
			entry: types.LockEntry{},
			want:  "",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.entry.EffectiveRef(); got != tc.want {
				t.Errorf("EffectiveRef() = %q, want %q", got, tc.want)
			}
		})
	}
}

// Lockfiles written before the version field existed are still readable.
func TestRead_AcceptsMissingVersion(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.lock")

	contents := "entries:\n  - name: legacy\n    source: https://example.com/a.git\n    content_hash: sha256:abc\n"
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	lock, err := Read(path)
	if err != nil {
		t.Fatalf("an unversioned lockfile should still load: %v", err)
	}
	if lock.Version != types.LockfileVersion {
		t.Errorf("Version = %q, want it stamped as %q", lock.Version, types.LockfileVersion)
	}
	if len(lock.Entries) != 1 {
		t.Errorf("expected the entry to survive, got %d", len(lock.Entries))
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.lock")

	lock := &types.Lockfile{
		Version: types.LockfileVersion,
		Entries: []types.LockEntry{
			{Name: "b-skill", Source: "src-b"},
			{Name: "a-skill", Source: "src-a"},
		},
	}

	if err := Write(path, lock); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	// Read it back
	readLock, err := Read(path)
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if readLock.GeneratedAt == "" {
		t.Error("GeneratedAt was not populated by Write()")
	}

	// Verify entries were sorted by name
	if len(readLock.Entries) != 2 {
		t.Fatalf("Entries length = %d", len(readLock.Entries))
	}
	if readLock.Entries[0].Name != "a-skill" || readLock.Entries[1].Name != "b-skill" {
		t.Errorf("Entries were not sorted correctly: %v", readLock.Entries)
	}
}

func TestWrite_PreservesLeadingNewlineInRef(t *testing.T) {
	t.Run("folded ref", func(t *testing.T) {
		dir := t.TempDir()
		in := filepath.Join(dir, "in.lock")
		// The nightly fuzzer's input: a folded scalar whose blank line is a
		// leading newline. The default encoder used to write it back as "0".
		const raw = "entries:\n- ref: >\n\n   0"
		if err := os.WriteFile(in, []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}

		lock, err := Read(in)
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		if len(lock.Entries) != 1 {
			t.Fatalf("entries = %d, want 1", len(lock.Entries))
		}
		if lock.Entries[0].Ref != "\n0" {
			t.Fatalf("Read() ref = %q, want a newline followed by 0", lock.Entries[0].Ref)
		}

		out := filepath.Join(dir, "out.lock")
		if err := Write(out, lock); err != nil {
			t.Fatalf("Write() error: %v", err)
		}

		again, err := Read(out)
		if err != nil {
			t.Fatalf("Read() after Write() error: %v", err)
		}
		if len(again.Entries) != 1 || again.Entries[0].Ref != "\n0" {
			t.Fatalf("ref after round trip = %+v, want a newline followed by 0", again.Entries)
		}
	})

	t.Run("empty optionals", func(t *testing.T) {
		dir := t.TempDir()
		in := filepath.Join(dir, "in.lock")
		const raw = "version: \"2\"\nentries:\n  - name: one\n    content_hash: sha256:abc\n"
		if err := os.WriteFile(in, []byte(raw), 0644); err != nil {
			t.Fatal(err)
		}

		lock, err := Read(in)
		if err != nil {
			t.Fatalf("Read() error: %v", err)
		}
		out := filepath.Join(dir, "out.lock")
		if err := Write(out, lock); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		again, err := Read(out)
		if err != nil {
			t.Fatalf("Read() after Write() error: %v", err)
		}
		if len(again.Entries) != 1 {
			t.Fatalf("entries = %d, want 1", len(again.Entries))
		}
		got := again.Entries[0]
		if got.Name != "one" || got.ContentHash != "sha256:abc" {
			t.Fatalf("entry = %+v", got)
		}
		if got.Source != "" || got.Path != "" || got.Ref != "" || got.ResolvedRef != "" || got.CommitSHA != "" {
			t.Fatalf("optional fields = %+v, want them empty", got)
		}
	})
}

func TestWrite_KeepsPlainPins(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.lock")
	const sha = "0123456789abcdef0123456789abcdef01234567"

	lock := &types.Lockfile{
		Version: types.LockfileVersion,
		Entries: []types.LockEntry{{
			Name:        "one",
			Source:      "https://example.com/one.git",
			ResolvedRef: "v1.1.0",
			CommitSHA:   sha,
			ContentHash: "sha256:abc",
			ResolvedAt:  "2026-01-01T00:00:00Z",
		}},
	}
	if err := Write(path, lock); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(body)
	if !strings.Contains(text, "resolved_ref: v1.1.0\n") {
		t.Fatalf("resolved ref was not a plain scalar:\n%s", text)
	}
	if !strings.Contains(text, "commit_sha: "+sha+"\n") {
		t.Fatalf("commit SHA was not a plain scalar:\n%s", text)
	}
	if !strings.Contains(text, "version: \"2\"\n") {
		t.Fatalf("version = missing quoted 2:\n%s", text)
	}
	if strings.Contains(text, "resolved_ref: \"") || strings.Contains(text, "commit_sha: \"") {
		t.Fatalf("plain pins were quoted:\n%s", text)
	}
}

func TestWrite_DoesNotReplaceFileWhenRoundTripChangesValue(t *testing.T) {
	lock := &types.Lockfile{
		Version:     "2",
		GeneratedAt: "2026-01-01T00:00:00Z",
		Entries: []types.LockEntry{{
			Name:        "one",
			Ref:         "\n0",
			ContentHash: "sha256:abc",
			ResolvedAt:  "2026-01-01T00:00:00Z",
		}},
	}
	// Same shape as a written lockfile, except the leading newline in ref is gone.
	bad := []byte("version: \"2\"\ngenerated_at: \"2026-01-01T00:00:00Z\"\nentries:\n  - name: one\n    ref: \"0\"\n    content_hash: sha256:abc\n    resolved_at: \"2026-01-01T00:00:00Z\"\n")

	dir := t.TempDir()
	existing := filepath.Join(dir, "skmgr.lock")
	const previous = "version: \"1\"\nstay: true\n"
	if err := os.WriteFile(existing, []byte(previous), 0644); err != nil {
		t.Fatal(err)
	}
	if err := writeIfUnchanged(existing, lock, bad); err == nil {
		t.Fatal("a document that changes a field was published, want an error")
	}
	got, err := os.ReadFile(existing)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != previous {
		t.Fatalf("existing file = %q, want it unchanged", got)
	}

	missing := filepath.Join(dir, "missing.lock")
	if err := writeIfUnchanged(missing, lock, bad); err == nil {
		t.Fatal("a document that changes a field created a file, want an error")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing destination stat = %v, want not exist", err)
	}
}

func TestHashDirectory(t *testing.T) {
	dir := t.TempDir()

	// Create some files
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("content1"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "file2.txt"), []byte("content2"), 0644); err != nil {
		t.Fatal(err)
	}

	hash1, err := HashDirectory(dir)
	if err != nil {
		t.Fatalf("HashDirectory() error: %v", err)
	}
	if hash1 == "" {
		t.Fatal("HashDirectory() returned empty hash")
	}

	// Verify deterministic: run again, should match
	hash2, err := HashDirectory(dir)
	if err != nil {
		t.Fatalf("HashDirectory() error: %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("Hash is not deterministic: %q != %q", hash1, hash2)
	}

	// Modify a file, hash should change
	if err := os.WriteFile(filepath.Join(dir, "file1.txt"), []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}
	hash3, err := HashDirectory(dir)
	if err != nil {
		t.Fatalf("HashDirectory() error: %v", err)
	}
	if hash1 == hash3 {
		t.Error("Hash did not change after modifying file contents")
	}
}

func TestHashDirectory_NotExist(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "does_not_exist")

	_, err := HashDirectory(path)
	if err == nil {
		t.Fatal("HashDirectory() expected error for non-existent directory, got nil")
	}
}
