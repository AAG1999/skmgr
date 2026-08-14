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

package engine

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestInstall_CopiesSkillToAgentsDir(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	_ = os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("data"), 0644)

	i := NewInstaller(root)
	rs := ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name: "test-skill",
			Type: types.TypeSkill,
		},
		SourceDir: source,
	}

	hash, err := i.Install(rs)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}
	if hash == "" {
		t.Error("Expected content hash, got empty string")
	}

	dest := filepath.Join(root, ".agents", "skills", "test-skill", "SKILL.md")
	if _, err := os.Stat(dest); os.IsNotExist(err) {
		t.Error("Skill was not copied to canonical directory")
	}
}

func TestInstall_UpdateReplacesExisting(t *testing.T) {
	root := t.TempDir()
	source := t.TempDir()
	_ = os.WriteFile(filepath.Join(source, "NEW.md"), []byte("new"), 0644)

	dest := filepath.Join(root, ".agents", "skills", "test-skill")
	_ = os.MkdirAll(dest, 0755)
	_ = os.WriteFile(filepath.Join(dest, "OLD.md"), []byte("old"), 0644)

	i := NewInstaller(root)
	rs := ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name: "test-skill",
		},
		SourceDir: source,
	}

	_, err := i.Install(rs)
	if err != nil {
		t.Fatalf("Install failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dest, "OLD.md")); !os.IsNotExist(err) {
		t.Error("Old files were not removed during update-in-place")
	}
	if _, err := os.Stat(filepath.Join(dest, "NEW.md")); os.IsNotExist(err) {
		t.Error("New files were not installed")
	}
}

func TestCleanOrphans_RemovesUnlisted(t *testing.T) {
	root := t.TempDir()
	skillsDir := filepath.Join(root, ".agents", "skills")
	_ = os.MkdirAll(filepath.Join(skillsDir, "valid-skill"), 0755)
	_ = os.MkdirAll(filepath.Join(skillsDir, "orphan-skill"), 0755)

	resolved := []ResolvedSkill{
		{
			SkillDependency: types.SkillDependency{Name: "valid-skill", Scope: types.ScopeProject},
		},
	}

	i := NewInstaller(root)
	if err := i.CleanOrphans(resolved, types.ScopeProject); err != nil {
		t.Fatalf("CleanOrphans failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(skillsDir, "valid-skill")); os.IsNotExist(err) {
		t.Error("Valid skill was incorrectly removed")
	}
	if _, err := os.Stat(filepath.Join(skillsDir, "orphan-skill")); !os.IsNotExist(err) {
		t.Error("Orphan skill was not removed")
	}
}
