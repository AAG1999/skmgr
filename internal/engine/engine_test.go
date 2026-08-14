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

func TestEngine_Sync_EndToEnd(t *testing.T) {
	root := t.TempDir()
	cache := t.TempDir()
	source := t.TempDir()
	skillsDir := filepath.Join(source, "skills", "my-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("hello"), 0644)

	e := NewEngine(root, cache)

	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{
				Name:   "my-skill",
				Source: "file://" + source,
				Type:   types.TypeSkill,
				Scope:  types.ScopeProject,
			},
		},
	}

	lock, err := e.Sync(manifest, nil, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if lock == nil || len(lock.Entries) != 1 {
		t.Fatal("Lockfile was not generated correctly")
	}

	// Verify it was copied to canonical dir
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "my-skill", "SKILL.md")); os.IsNotExist(err) {
		t.Error("Skill not installed in canonical directory")
	}

	// Verify it was linked
	link, _ := os.Readlink(filepath.Join(root, ".cursor", "skills", "my-skill"))
	if link == "" {
		t.Error("Symlink for cursor not created")
	}
}

func TestEngine_Remove(t *testing.T) {
	root := t.TempDir()
	e := NewEngine(root, t.TempDir())

	// Create dummy symlink
	agentDir := filepath.Join(root, ".cursor", "skills")
	_ = os.MkdirAll(agentDir, 0755)
	_ = os.Symlink("dummy", filepath.Join(agentDir, "my-skill"))

	if err := e.Remove("my-skill", types.ScopeProject, []string{"cursor"}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	if _, err := os.Lstat(filepath.Join(agentDir, "my-skill")); !os.IsNotExist(err) {
		t.Error("Symlink was not removed")
	}
}
