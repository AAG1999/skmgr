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

func TestMerge_InsertIntoEmptyFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")

	content := "Be helpful."
	if err := mergeSection(file, "test-rule", content); err != nil {
		t.Fatalf("mergeSection failed: %v", err)
	}

	data, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	str := string(data)

	if !strings.Contains(str, "<!-- skmgr:start:test-rule -->") {
		t.Error("Missing start delimiter")
	}
	if !strings.Contains(str, content) {
		t.Error("Missing content")
	}
	if !strings.Contains(str, "<!-- skmgr:end:test-rule -->") {
		t.Error("Missing end delimiter")
	}
}

func TestMerge_InsertIntoExistingFile(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")
	_ = os.WriteFile(file, []byte("User content here.\n"), 0644)

	content := "Be concise."
	if err := mergeSection(file, "test-rule", content); err != nil {
		t.Fatalf("mergeSection failed: %v", err)
	}

	data, _ := os.ReadFile(file)
	str := string(data)

	// Should preserve existing content at the top
	if !strings.HasPrefix(str, "User content here.\n") {
		t.Errorf("User content was not preserved correctly. Got:\n%s", str)
	}
	if !strings.Contains(str, content) {
		t.Error("Missing new content")
	}
}

func TestMerge_UpdateExistingSection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")
	_ = os.WriteFile(file, []byte("User content\n<!-- skmgr:start:rule1 -->\nOld content\n<!-- skmgr:end:rule1 -->\nFooter\n"), 0644)

	newContent := "New content"
	if err := mergeSection(file, "rule1", newContent); err != nil {
		t.Fatalf("mergeSection failed: %v", err)
	}

	data, _ := os.ReadFile(file)
	str := string(data)

	if strings.Contains(str, "Old content") {
		t.Error("Old content was not removed")
	}
	if !strings.Contains(str, "New content") {
		t.Error("New content was not inserted")
	}
	if !strings.Contains(str, "User content") || !strings.Contains(str, "Footer") {
		t.Error("Surrounding user content was altered")
	}
}

func TestMerge_MultipleSections(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")

	_ = mergeSection(file, "rule1", "Content 1")
	_ = mergeSection(file, "rule2", "Content 2")

	data, _ := os.ReadFile(file)
	str := string(data)

	if !strings.Contains(str, "<!-- skmgr:start:rule1 -->") || !strings.Contains(str, "Content 1") {
		t.Error("Rule 1 missing")
	}
	if !strings.Contains(str, "<!-- skmgr:start:rule2 -->") || !strings.Contains(str, "Content 2") {
		t.Error("Rule 2 missing")
	}
}

func TestMerge_PreservesUserContent(t *testing.T) {
	// Already tested implicitly in UpdateExistingSection and InsertIntoExistingFile,
	// but adding for completion as per plan.
}

func TestRemoveSection_Basic(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")
	_ = os.WriteFile(file, []byte("User content\n<!-- skmgr:start:rule1 -->\nManaged content\n<!-- skmgr:end:rule1 -->\nFooter\n"), 0644)

	if err := removeSection(file, "rule1"); err != nil {
		t.Fatalf("removeSection failed: %v", err)
	}

	data, _ := os.ReadFile(file)
	str := string(data)

	if strings.Contains(str, "skmgr:start:rule1") || strings.Contains(str, "Managed content") {
		t.Error("Section was not removed")
	}
	if !strings.Contains(str, "User content") || !strings.Contains(str, "Footer") {
		t.Error("User content was affected")
	}
}

func TestRemoveSection_LastSection(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")
	_ = os.WriteFile(file, []byte("User content\n<!-- skmgr:start:rule1 -->\nManaged content\n<!-- skmgr:end:rule1 -->\n"), 0644)

	_ = removeSection(file, "rule1")
	data, _ := os.ReadFile(file)
	str := string(data)

	if strings.Contains(str, "Managed") {
		t.Error("Failed to remove last section")
	}
	if !strings.Contains(str, "User content") {
		t.Error("User content damaged")
	}
}

func TestRemoveSection_NotFound(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")
	_ = os.WriteFile(file, []byte("Just some content\n"), 0644)

	if err := removeSection(file, "nonexistent"); err != nil {
		t.Fatalf("removeSection failed on nonexistent section: %v", err)
	}

	data, _ := os.ReadFile(file)
	if string(data) != "Just some content\n" {
		t.Error("File content was altered when section not found")
	}
}

func TestRemoveSection_FileNotExists(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "rules.md")

	if err := removeSection(file, "rule1"); err != nil {
		t.Fatalf("removeSection should succeed (no-op) if file doesn't exist, got: %v", err)
	}
}
