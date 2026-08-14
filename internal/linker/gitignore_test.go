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
	"strings"
	"testing"
)

func TestGitignore_AddEntry_NewFile(t *testing.T) {
	dir := t.TempDir()

	if err := ensureGitignoreEntry(dir, ".agents/skills/foo"); err != nil {
		t.Fatalf("ensureGitignoreEntry failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}

	str := string(data)
	if !strings.Contains(str, "# skmgr:managed") {
		t.Error("Missing start delim")
	}
	if !strings.Contains(str, ".agents/skills/foo") {
		t.Error("Missing path")
	}
	if !strings.Contains(str, "# skmgr:end") {
		t.Error("Missing end delim")
	}
}

func TestGitignore_AddEntry_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitignore")
	_ = os.WriteFile(file, []byte("node_modules/\n"), 0644)

	_ = ensureGitignoreEntry(dir, ".cursor/skills/foo")

	data, _ := os.ReadFile(file)
	str := string(data)

	if !strings.HasPrefix(str, "node_modules/") {
		t.Error("User content altered")
	}
	if !strings.Contains(str, ".cursor/skills/foo") {
		t.Error("Missing path")
	}
}

func TestGitignore_AddEntry_ExistingSection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitignore")
	_ = os.WriteFile(file, []byte("node_modules/\n# skmgr:managed\n.cursor/skills/foo\n# skmgr:end\n"), 0644)

	_ = ensureGitignoreEntry(dir, ".cursor/skills/bar")

	data, _ := os.ReadFile(file)
	str := string(data)

	if !strings.Contains(str, ".cursor/skills/foo") || !strings.Contains(str, ".cursor/skills/bar") {
		t.Error("Both paths should be present")
	}
}

func TestGitignore_AddEntry_Duplicate(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitignore")
	_ = os.WriteFile(file, []byte("# skmgr:managed\n.cursor/skills/foo\n# skmgr:end\n"), 0644)

	_ = ensureGitignoreEntry(dir, ".cursor/skills/foo")

	data, _ := os.ReadFile(file)
	str := string(data)

	if strings.Count(str, ".cursor/skills/foo") > 1 {
		t.Error("Duplicated entry found")
	}
}

func TestGitignore_RemoveEntry(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitignore")
	_ = os.WriteFile(file, []byte("# skmgr:managed\n.cursor/skills/foo\n.cursor/skills/bar\n# skmgr:end\n"), 0644)

	if err := removeGitignoreEntry(dir, ".cursor/skills/foo"); err != nil {
		t.Fatalf("removeGitignoreEntry failed: %v", err)
	}

	data, _ := os.ReadFile(file)
	str := string(data)

	if strings.Contains(str, ".cursor/skills/foo") {
		t.Error("Path was not removed")
	}
	if !strings.Contains(str, ".cursor/skills/bar") {
		t.Error("Other path was removed incorrectly")
	}
}

func TestGitignore_RemoveEntry_LastOne(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitignore")
	_ = os.WriteFile(file, []byte("node_modules/\n# skmgr:managed\n.cursor/skills/foo\n# skmgr:end\n"), 0644)

	_ = removeGitignoreEntry(dir, ".cursor/skills/foo")

	data, _ := os.ReadFile(file)
	str := string(data)

	if strings.Contains(str, "# skmgr:managed") {
		t.Error("Managed section was not removed when empty")
	}
	if !strings.Contains(str, "node_modules/") {
		t.Error("User content was affected")
	}
}

func TestGitignore_PreservesUserEntries(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, ".gitignore")
	_ = os.WriteFile(file, []byte("dist/\n# skmgr:managed\n.cursor/skills/foo\n# skmgr:end\nbuild/\n"), 0644)

	_ = removeGitignoreEntry(dir, ".cursor/skills/foo")

	data, _ := os.ReadFile(file)
	str := string(data)

	if !strings.Contains(str, "dist/") || !strings.Contains(str, "build/") {
		t.Error("User content was modified")
	}
}

func TestGitignore_Rebuild(t *testing.T) {
	// Rebuild logic is covered by ensuring deterministic sort and injection
	dir := t.TempDir()
	_ = ensureGitignoreEntry(dir, "b")
	_ = ensureGitignoreEntry(dir, "c")
	_ = ensureGitignoreEntry(dir, "a")

	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	str := string(data)

	if !strings.Contains(str, "a\nb\nc") {
		t.Error("Entries were not sorted alphabetically")
	}
}
