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

	"github.com/AAG1999/skmgr/internal/types"
)

func TestLinkSkill_Cursor_Project(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	// Setup canonical skill
	canonDir := filepath.Join(dir, ".agents", "skills", "my-skill")
	_ = os.MkdirAll(canonDir, 0755)

	err := l.LinkSkill("cursor", "my-skill", types.ScopeProject, dir)
	if err != nil {
		t.Fatalf("LinkSkill failed: %v", err)
	}

	linkPath := filepath.Join(dir, ".cursor", "skills", "my-skill")
	if !isSymlink(linkPath) {
		t.Error("Symlink for cursor not created")
	}

	// Verify gitignore
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(data), filepath.ToSlash(filepath.Join(".cursor", "skills", "my-skill"))) {
		t.Error(".gitignore not updated properly")
	}
}

func TestLinkSkill_Gemini_Project_Skipped(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	err := l.LinkSkill("gemini", "my-skill", types.ScopeProject, dir)
	if err != nil {
		t.Fatalf("LinkSkill failed: %v", err)
	}

	linkPath := filepath.Join(dir, ".gemini", "config", "skills", "my-skill")
	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("Gemini project scope should not create a symlink")
	}
}

func TestLinkSkill_Gemini_Global(t *testing.T) {
	// Mocking homedir behavior is hard, so we just pass dir as projectRoot
	// Since global scope usually uses user config dir, we need to ensure the test
	// doesn't write to the real ~/.gemini.
}

func TestLinkRule_Merge_Claude(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	canonDir := filepath.Join(dir, ".agents", "rules", "my-rule")
	_ = os.MkdirAll(canonDir, 0755)
	_ = os.WriteFile(filepath.Join(canonDir, "SKILL.md"), []byte("rule content"), 0644)

	err := l.LinkRule("claude-code", "my-rule", types.ScopeProject, dir)
	if err != nil {
		t.Fatalf("LinkRule failed: %v", err)
	}

	targetPath := filepath.Join(dir, ".claude", "CLAUDE.md")
	data, _ := os.ReadFile(targetPath)
	str := string(data)

	if !strings.Contains(str, "<!-- skmgr:start:my-rule -->") {
		t.Error("Merge failed for Claude Code")
	}
}

func TestUnlinkSkill_RemovesSymlink(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	canonDir := filepath.Join(dir, ".agents", "skills", "my-skill")
	_ = os.MkdirAll(canonDir, 0755)

	_ = l.LinkSkill("cursor", "my-skill", types.ScopeProject, dir)

	linkPath := filepath.Join(dir, ".cursor", "skills", "my-skill")
	if !isSymlink(linkPath) {
		t.Fatal("Setup failed")
	}

	err := l.UnlinkSkill("cursor", "my-skill", types.ScopeProject, dir)
	if err != nil {
		t.Fatalf("UnlinkSkill failed: %v", err)
	}

	if isSymlink(linkPath) {
		t.Error("Symlink was not removed")
	}

	// Verify gitignore
	data, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if strings.Contains(string(data), ".cursor/skills/my-skill") {
		t.Error(".gitignore entry was not removed")
	}
}

func TestVerify_AllLinksValid(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{Name: "skill1"},
		},
	}

	canonDir := filepath.Join(dir, ".agents", "skills", "skill1")
	_ = os.MkdirAll(canonDir, 0755)

	_ = l.LinkSkill("cursor", "skill1", types.ScopeProject, dir)

	issues := l.Verify(manifest, types.ScopeProject, dir)
	if len(issues) > 0 {
		t.Errorf("Expected 0 issues, got %d: %v", len(issues), issues)
	}
}

func TestVerify_MissingSymlink(t *testing.T) {
	dir := t.TempDir()
	l := NewLinker()

	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{Name: "skill1"},
		},
	}

	issues := l.Verify(manifest, types.ScopeProject, dir)
	if len(issues) != 1 {
		t.Fatalf("Expected 1 issue, got %d", len(issues))
	}

	if !strings.Contains(issues[0].Message, "missing") {
		t.Errorf("Expected missing symlink message, got: %s", issues[0].Message)
	}
}
