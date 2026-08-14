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
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// buildTree writes a directory of the given shape and returns its root.
func buildTree(b *testing.B, files int, depth int, size int) string {
	b.Helper()

	root := b.TempDir()
	content := make([]byte, size)
	for i := range content {
		content[i] = byte('a' + i%26)
	}

	dir := root
	for level := 0; level < depth; level++ {
		dir = filepath.Join(dir, fmt.Sprintf("level%d", level))
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		b.Fatal(err)
	}

	for i := 0; i < files; i++ {
		path := filepath.Join(dir, fmt.Sprintf("file%04d.md", i))
		if err := os.WriteFile(path, content, 0644); err != nil {
			b.Fatal(err)
		}
	}

	return root
}

// BenchmarkHashDirectory measures the cost of the integrity check.
//
// This runs once per skill on install, verify, list and doctor, so it is on the
// path of every command a person waits for. The shapes below bracket what a
// real skill looks like: a handful of markdown files, a large documentation
// set, and one big file.
func BenchmarkHashDirectory(b *testing.B) {
	shapes := []struct {
		name  string
		files int
		depth int
		size  int
	}{
		{"typical_10_files_4KB", 10, 2, 4 << 10},
		{"large_500_files_4KB", 500, 3, 4 << 10},
		{"deep_50_files_20_levels", 50, 20, 1 << 10},
		{"one_10MB_file", 1, 1, 10 << 20},
	}

	for _, shape := range shapes {
		b.Run(shape.name, func(b *testing.B) {
			root := buildTree(b, shape.files, shape.depth, shape.size)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := HashDirectory(root); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// syntheticLock builds a lockfile with the given number of entries.
func syntheticLock(entries int) *types.Lockfile {
	lock := types.NewLockfile()

	for i := 0; i < entries; i++ {
		lock.Entries = append(lock.Entries, types.LockEntry{
			Name:        fmt.Sprintf("skill-%04d", i),
			Source:      "https://github.com/example/skills.git",
			Path:        fmt.Sprintf("skills/skill-%04d", i),
			Ref:         "^1.2.0",
			ResolvedRef: "v1.4.2",
			CommitSHA:   "0123456789abcdef0123456789abcdef01234567",
			ContentHash: "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			ResolvedAt:  "2026-01-01T00:00:00Z",
		})
	}

	return lock
}

// BenchmarkRead measures loading a lockfile, which happens at the start of
// every command.
func BenchmarkRead(b *testing.B) {
	for _, entries := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("%d_entries", entries), func(b *testing.B) {
			path := filepath.Join(b.TempDir(), "skmgr.lock")
			if err := Write(path, syntheticLock(entries)); err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := Read(path); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// BenchmarkWrite measures the sort-encode-fsync-rename cycle.
func BenchmarkWrite(b *testing.B) {
	for _, entries := range []int{10, 100, 1000} {
		b.Run(fmt.Sprintf("%d_entries", entries), func(b *testing.B) {
			lock := syntheticLock(entries)
			path := filepath.Join(b.TempDir(), "skmgr.lock")

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if err := Write(path, lock); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
