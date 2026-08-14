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

package linker

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestCreateSymlink_Basic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	// Create a dummy target
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := createSymlink(target, link); err != nil {
		t.Fatalf("createSymlink failed: %v", err)
	}

	// Verify it's a symlink
	if !isSymlink(link) {
		t.Error("isSymlink returned false for created symlink")
	}

	// The link is stored relative to its own directory so the project can move
	// without every link going stale.
	resolved, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink failed: %v", err)
	}
	if resolved != "target.txt" {
		t.Errorf("Resolved to %q, want the relative %q", resolved, "target.txt")
	}

	// It still has to reach the file it names.
	if got, err := linkTarget(link); err != nil || got != target {
		t.Errorf("linkTarget = (%q, %v), want %q", got, err, target)
	}
}

func TestCreateSymlink_CreatesParentDirs(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	// Target doesn't strictly need to exist for os.Symlink, but let's make it
	if err := os.WriteFile(target, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	link := filepath.Join(dir, "deep", "nested", "dir", "link.txt")

	if err := createSymlink(target, link); err != nil {
		t.Fatalf("createSymlink failed: %v", err)
	}
	if !isSymlink(link) {
		t.Error("Link was not created properly")
	}
}

func TestCreateSymlink_AlreadyExists_SameTarget(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")
	_ = os.WriteFile(target, nil, 0644)

	// Create first time
	if err := createSymlink(target, link); err != nil {
		t.Fatal(err)
	}

	// Create second time - should be no-op success
	if err := createSymlink(target, link); err != nil {
		t.Fatalf("createSymlink second time failed: %v", err)
	}
}

func TestCreateSymlink_AlreadyExists_DifferentTarget(t *testing.T) {
	dir := t.TempDir()
	target1 := filepath.Join(dir, "target1.txt")
	target2 := filepath.Join(dir, "target2.txt")
	link := filepath.Join(dir, "link.txt")

	// Create first pointing to target1
	if err := createSymlink(target1, link); err != nil {
		t.Fatal(err)
	}

	// Create again pointing to target2
	if err := createSymlink(target2, link); err != nil {
		t.Fatalf("createSymlink with new target failed: %v", err)
	}

	// Verify it points to target2
	resolved, _ := os.Readlink(link)
	if resolved != "target2.txt" {
		t.Errorf("Resolved to %q, want %q", resolved, "target2.txt")
	}
}

func TestCreateSymlink_RegularFileExists(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link.txt")

	// Make the "link" path a regular file
	if err := os.WriteFile(link, []byte("i am a real file"), 0644); err != nil {
		t.Fatal(err)
	}

	err := createSymlink(target, link)
	if err == nil {
		t.Fatal("createSymlink should fail if a regular file exists at link path")
	}
}

func TestIsSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	regFile := filepath.Join(dir, "file")

	_ = os.WriteFile(regFile, nil, 0644)
	_ = os.Symlink(regFile, link)

	if !isSymlink(link) {
		t.Error("isSymlink(link) = false, want true")
	}
	if isSymlink(regFile) {
		t.Error("isSymlink(regFile) = true, want false")
	}
	if isSymlink(filepath.Join(dir, "nonexistent")) {
		t.Error("isSymlink(nonexistent) = true, want false")
	}
}

func TestRemoveSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "link")
	_ = os.Symlink("foo", link) // broken symlink is fine

	// Remove it
	if err := removeSymlink(link); err != nil {
		t.Fatalf("removeSymlink failed: %v", err)
	}
	if isSymlink(link) {
		t.Error("symlink still exists after removal")
	}

	// Remove again (should be no-op success)
	if err := removeSymlink(link); err != nil {
		t.Fatalf("removeSymlink on nonexistent failed: %v", err)
	}
}

func TestRemoveSymlink_RefusesRegularFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "file.txt")
	_ = os.WriteFile(file, nil, 0644)

	err := removeSymlink(file)
	if err == nil {
		t.Fatal("removeSymlink should fail on regular files")
	}
	// Verify file is still there
	if _, err := os.Stat(file); os.IsNotExist(err) {
		t.Error("removeSymlink incorrectly deleted a regular file")
	}
}

func TestCleanBrokenLinks(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	// Set up a broken link and a valid link in the cursor skills dir
	// We use the default 'cursor' agent definition
	cursorDir := filepath.Join(dir, ".cursor", "skills")
	if err := os.MkdirAll(cursorDir, 0755); err != nil {
		t.Fatal(err)
	}

	validTarget := filepath.Join(dir, "valid-target")
	_ = os.WriteFile(validTarget, []byte(""), 0644)

	validLink := filepath.Join(cursorDir, "valid-link")
	_ = os.Symlink(validTarget, validLink)

	brokenLink := filepath.Join(cursorDir, "broken-link")
	_ = os.Symlink(filepath.Join(dir, "non-existent"), brokenLink)

	if err := l.CleanBrokenLinks(types.ScopeProject, dir); err != nil {
		t.Fatalf("CleanBrokenLinks failed: %v", err)
	}

	if !isSymlink(validLink) {
		t.Error("Valid link was removed")
	}

	if isSymlink(brokenLink) {
		t.Error("Broken link was not removed")
	}
}
