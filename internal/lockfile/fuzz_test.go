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
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// lockSeeds cover every shape the reader has to cope with: the two supported
// versions, the unversioned files that predate versioning, a version from the
// future, and a handful of ways the YAML can be wrong.
var lockSeeds = []string{
	"",
	"\n",
	"---\n",
	"version: \"2\"\n",
	"version: \"1\"\nentries: []\n",
	"version: \"2\"\ngenerated_at: 2026-01-01T00:00:00Z\nentries:\n  - name: one\n    source: ./a\n    content_hash: sha256:abc\n    resolved_at: 2026-01-01T00:00:00Z\n",
	"version: \"2\"\nentries:\n  - name: one\n    source: ./a\n    ref: ^1.0.0\n    resolved_ref: v1.4.2\n    commit_sha: deadbeef\n    content_hash: sha256:abc\n",
	"generated_at: 2026-01-01T00:00:00Z\nentries:\n  - name: legacy\n    content_hash: sha256:abc\n",
	"version: \"99\"\n",
	"version: 2\n",
	"version: [2]\n",
	"entries: not-a-list\n",
	"entries:\n  - 5\n",
	"a: &x [*x]\n",
	"version: \"2\"\nentries:\n  - name: \"../escape\"\n    content_hash: sha256:abc\n",
}

// FuzzRead checks that no lockfile on disk can make the reader panic, and that
// a file it accepts is one the rest of skmgr can act on.
//
// The reader is what stands between a `skmgr.lock` pulled from a branch and
// code that installs from it, so the property worth asserting is not "bad input
// errors" but "accepted input is coherent".
func FuzzRead(f *testing.F) {
	for _, seed := range lockSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		path := filepath.Join(t.TempDir(), "skmgr.lock")
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Skip("could not stage the input")
		}

		lock, err := Read(path)
		if err != nil {
			if lock != nil {
				t.Fatal("a failed read must not return a lockfile")
			}
			return
		}

		if lock == nil {
			t.Fatal("a successful read must return a lockfile")
		}

		// The reader normalizes every version it accepts to the current one, so
		// a caller never has to branch on the format it came from. If an old
		// version leaked through unchanged, the next write would silently
		// preserve it.
		if lock.Version != types.LockfileVersion {
			t.Fatalf("accepted version %q, want it normalized to %q (input %q)",
				lock.Version, types.LockfileVersion, data)
		}
	})
}

// FuzzReadWriteRoundTrip checks that anything the reader accepts survives being
// written back out and read again.
//
// Every command that touches a lockfile does exactly this: read, adjust, write.
// A value that changes meaning on the way through would quietly repin a skill
// to something nobody chose.
func FuzzReadWriteRoundTrip(f *testing.F) {
	for _, seed := range lockSeeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		dir := t.TempDir()
		first := filepath.Join(dir, "first.lock")
		if err := os.WriteFile(first, data, 0644); err != nil {
			t.Skip("could not stage the input")
		}

		original, err := Read(first)
		if err != nil {
			return
		}

		second := filepath.Join(dir, "second.lock")
		if err := Write(second, original); err != nil {
			t.Fatalf("a lockfile that was read must be writable: %v (input %q)", err, data)
		}

		reread, err := Read(second)
		if err != nil {
			t.Fatalf("a lockfile skmgr wrote must be readable: %v (input %q)", err, data)
		}

		if !slices.Equal(entryNames(original), entryNames(reread)) {
			t.Fatalf("entry names changed across a round trip: %v then %v (input %q)",
				entryNames(original), entryNames(reread), data)
		}

		for _, name := range entryNames(original) {
			before, after := original.FindEntry(name), reread.FindEntry(name)
			if after == nil {
				t.Fatalf("entry %q vanished across a round trip (input %q)", name, data)
			}
			if *before != *after {
				t.Fatalf("entry %q changed across a round trip:\n before %+v\n after  %+v\n(input %q)",
					name, *before, *after, data)
			}
		}
	})
}

// entryNames lists the entry names in the order they are stored.
func entryNames(lock *types.Lockfile) []string {
	names := make([]string, 0, len(lock.Entries))
	for _, entry := range lock.Entries {
		names = append(names, entry.Name)
	}
	slices.Sort(names)
	return names
}
