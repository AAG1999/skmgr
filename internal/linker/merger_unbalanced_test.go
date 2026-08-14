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

// A start marker with no matching end used to make the rewrite swallow every
// line that followed it. The merge must refuse instead of destroying content.
func TestMergeSection_RefusesOnMissingEndDelimiter(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	original := strings.Join([]string{
		"# My project instructions",
		"",
		"<!-- skmgr:start:my-rule -->",
		"old managed content",
		"",
		"## Hand written notes that must survive",
		"These lines come after the orphaned start marker.",
	}, "\n")

	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	err := mergeSection(target, "my-rule", "new content")
	if err == nil {
		t.Fatal("mergeSection() succeeded on unbalanced delimiters, want an error")
	}
	if !strings.Contains(err.Error(), "missing its closing") {
		t.Errorf("error = %q, want it to explain the missing closing marker", err.Error())
	}

	after, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatalf("reading target after failed merge: %v", readErr)
	}
	if string(after) != original {
		t.Error("mergeSection() modified the file despite failing; content must be left untouched")
	}
}

func TestMergeSection_RefusesOnStrayEndDelimiter(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	original := "# Notes\n<!-- skmgr:end:my-rule -->\nmore notes\n"
	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := mergeSection(target, "my-rule", "new content"); err == nil {
		t.Fatal("mergeSection() succeeded with a stray end marker, want an error")
	}

	after, _ := os.ReadFile(target)
	if string(after) != original {
		t.Error("mergeSection() modified the file despite failing")
	}
}

func TestRemoveSection_RefusesOnMissingEndDelimiter(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	original := "# Notes\n<!-- skmgr:start:my-rule -->\ncontent\ntrailing user text\n"
	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := removeSection(target, "my-rule"); err == nil {
		t.Fatal("removeSection() succeeded on unbalanced delimiters, want an error")
	}

	after, _ := os.ReadFile(target)
	if string(after) != original {
		t.Error("removeSection() modified the file despite failing")
	}
}

func TestMergeSection_BalancedStillWorks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "CLAUDE.md")

	original := strings.Join([]string{
		"# Project",
		"",
		"<!-- skmgr:start:my-rule -->",
		"old content",
		"<!-- skmgr:end:my-rule -->",
		"",
		"## Trailing notes",
	}, "\n")

	if err := os.WriteFile(target, []byte(original), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := mergeSection(target, "my-rule", "new content"); err != nil {
		t.Fatalf("mergeSection() returned error on balanced input: %v", err)
	}

	after, _ := os.ReadFile(target)
	got := string(after)

	if !strings.Contains(got, "new content") {
		t.Error("merged file is missing the new content")
	}
	if strings.Contains(got, "old content") {
		t.Error("merged file still contains the replaced content")
	}
	if !strings.Contains(got, "## Trailing notes") {
		t.Error("merged file lost user content that followed the managed section")
	}
}

func TestEnsureGitignoreEntry_RefusesOnMissingEndDelimiter(t *testing.T) {
	dir := t.TempDir()
	gitignore := filepath.Join(dir, ".gitignore")

	original := "node_modules/\n# skmgr:managed\n.cursor/skills/foo\ndist/\n"
	if err := os.WriteFile(gitignore, []byte(original), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	if err := ensureGitignoreEntry(dir, ".cursor/skills/bar"); err == nil {
		t.Fatal("ensureGitignoreEntry() succeeded on unbalanced markers, want an error")
	}

	after, _ := os.ReadFile(gitignore)
	if string(after) != original {
		t.Error("ensureGitignoreEntry() rewrote .gitignore despite failing; dist/ would have been lost")
	}
}
