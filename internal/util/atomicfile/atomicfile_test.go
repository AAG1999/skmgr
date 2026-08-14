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

package atomicfile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := Write(path, []byte("hello"), 0644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" {
		t.Errorf("contents = %q, want %q", got, "hello")
	}
}

func TestWrite_ReplacesExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := os.WriteFile(path, []byte("old content that is long"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := Write(path, []byte("new"), 0644); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Errorf("contents = %q, want %q", got, "new")
	}
}

func TestWrite_CreatesParentDirectories(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nested", "deeper", "out.txt")

	if err := Write(path, []byte("x"), 0644); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Errorf("file was not created: %v", err)
	}
}

func TestWrite_AppliesPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permission bits are not meaningful on Windows")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	if err := Write(path, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// CreateTemp makes the file 0600, so a passing 0644 case is the one that
	// actually proves the chmod happened.
	if got := info.Mode().Perm(); got != 0600 {
		t.Errorf("mode = %v, want 0600", got)
	}

	if err := Write(path, []byte("y"), 0644); err != nil {
		t.Fatal(err)
	}
	info, err = os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0644 {
		t.Errorf("mode = %v, want 0644", got)
	}
}

// The whole point of the package: a failed write must not damage what is
// already on disk. The failure is simulated by making the rename impossible.
func TestWrite_FailureLeavesOriginalIntact(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("directory permissions do not block renames the same way on Windows")
	}
	if os.Geteuid() == 0 {
		t.Skip("root ignores directory permissions")
	}

	dir := t.TempDir()
	sub := filepath.Join(dir, "locked")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(sub, "out.txt")
	original := "the original content"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	// A read-only directory blocks creating the temporary file.
	if err := os.Chmod(sub, 0500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sub, 0755) })

	if err := Write(path, []byte("replacement"), 0644); err == nil {
		t.Fatal("expected Write to fail in a read-only directory")
	}

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original {
		t.Errorf("original was damaged: %q", got)
	}
}

// A reader must never observe a truncated file, which is exactly what
// os.WriteFile allows while it is mid-write.
func TestWrite_ReaderNeverSeesPartialContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	small := strings.Repeat("a", 64)
	large := strings.Repeat("b", 1<<20)

	if err := Write(path, []byte(small), 0644); err != nil {
		t.Fatal(err)
	}

	var wg sync.WaitGroup
	stop := make(chan struct{})

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				if os.IsNotExist(err) {
					t.Error("the target briefly did not exist")
					return
				}
				continue
			}
			s := string(data)
			if s != small && s != large {
				t.Errorf("observed a partial file of length %d", len(s))
				return
			}
		}
	}()

	for i := 0; i < 50; i++ {
		payload := large
		if i%2 == 0 {
			payload = small
		}
		if err := Write(path, []byte(payload), 0644); err != nil {
			t.Errorf("Write: %v", err)
			break
		}
	}

	close(stop)
	wg.Wait()
}

// Temporary files are created next to the target, so a failure must not leave
// them lying around for the user to find.
func TestWrite_LeavesNoTemporaryFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")

	for i := 0; i < 5; i++ {
		if err := Write(path, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the target file, found %v", names)
	}
}
