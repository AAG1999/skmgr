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

func projectSkill(source, name, scope string) types.SkillDependency {
	return types.SkillDependency{
		Name:   name,
		Source: "file://" + source,
		Path:   filepath.Join("skills", name),
		Scope:  scope,
	}
}

func writeDirFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

// A directory skmgr never recorded must survive a sync, including the first
// one, when there is no lockfile to name it.
func TestSync_KeepsUnmanagedProjectDirectories(t *testing.T) {
	isolateHome(t)

	root := t.TempDir()
	skill := filepath.Join(root, ".agents", "skills", "other-tool-skill")
	rule := filepath.Join(root, ".agents", "rules", "other-tool-rule")
	marker := filepath.Join(root, ".agents", "skills", ".openspec-target")
	writeDirFile(t, skill, "SKILL.md", "other tool")
	writeDirFile(t, rule, "RULE.md", "other rule")
	writeDirFile(t, filepath.Dir(marker), ".openspec-target", "marker")

	source := t.TempDir()
	writeSkillSource(t, source, "local-skill")

	e := newTestEngine(t, root, t.TempDir())
	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills:  []types.SkillDependency{projectSkill(source, "local-skill", types.ScopeProject)},
	}
	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	for _, path := range []string{
		filepath.Join(skill, "SKILL.md"),
		filepath.Join(rule, "RULE.md"),
		marker,
		filepath.Join(root, ".agents", "skills", "local-skill", "SKILL.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Errorf("expected %s to remain: %v", path, err)
		}
	}
}

// A name the previous lock recorded and the manifest no longer resolves is
// removed from both canonical trees. The lock has no type, so the rules copy
// goes with the skills copy.
func TestSync_RemovesDroppedLockEntryFromBothTrees(t *testing.T) {
	isolateHome(t)

	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "keep-me")
	writeSkillSource(t, source, "drop-me")

	e := newTestEngine(t, root, t.TempDir())
	both := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			projectSkill(source, "keep-me", types.ScopeProject),
			projectSkill(source, "drop-me", types.ScopeProject),
		},
	}
	lock, err := e.Sync(both, nil, SyncOptions{})
	if err != nil {
		t.Fatalf("first Sync failed: %v", err)
	}

	// Install of a skill removes the same name under rules, so the rules copy
	// has to appear after that sync for the second pass to have something to delete.
	rules := filepath.Join(root, ".agents", "rules", "drop-me")
	writeDirFile(t, rules, "RULE.md", "stale")

	keepOnly := &types.Manifest{
		Targets: []string{"cursor"},
		Skills:  []types.SkillDependency{projectSkill(source, "keep-me", types.ScopeProject)},
	}
	if _, err := e.Sync(keepOnly, lock, SyncOptions{}); err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "drop-me")); !os.IsNotExist(err) {
		t.Error("drop-me should have been removed from .agents/skills")
	}
	if _, err := os.Stat(rules); !os.IsNotExist(err) {
		t.Error("drop-me should have been removed from .agents/rules")
	}
	if _, err := os.Stat(filepath.Join(root, ".agents", "skills", "keep-me", "SKILL.md")); err != nil {
		t.Errorf("keep-me should still be installed: %v", err)
	}
}

// Moving a skill to global scope drops the project directory. Leaving it would
// keep the old project symlink pointing at a tree the manifest no longer uses.
func TestSync_ScopeChangeRemovesProjectDirectory(t *testing.T) {
	home := isolateHome(t)

	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "moved")

	e := newTestEngine(t, root, t.TempDir())
	asProject := &types.Manifest{
		Targets: []string{"cursor"},
		Skills:  []types.SkillDependency{projectSkill(source, "moved", types.ScopeProject)},
	}
	lock, err := e.Sync(asProject, nil, SyncOptions{})
	if err != nil {
		t.Fatalf("project Sync failed: %v", err)
	}
	projectDir := filepath.Join(root, ".agents", "skills", "moved")
	if _, err := os.Stat(projectDir); err != nil {
		t.Fatalf("project install did not create %s: %v", projectDir, err)
	}

	asGlobal := &types.Manifest{
		Targets: []string{"cursor"},
		Skills:  []types.SkillDependency{projectSkill(source, "moved", types.ScopeGlobal)},
	}
	if _, err := e.Sync(asGlobal, lock, SyncOptions{}); err != nil {
		t.Fatalf("global Sync failed: %v", err)
	}

	if _, err := os.Stat(projectDir); !os.IsNotExist(err) {
		t.Error("project directory should be removed when the skill moves to global scope")
	}
	homeSkill := filepath.Join(home, ".agents", "skills", "moved", "SKILL.md")
	if _, err := os.Stat(homeSkill); err != nil {
		t.Errorf("skill should be installed under the home .agents tree: %v", err)
	}
}

// A reserved workflow directory stays even when a hand-edited lock names it and
// the manifest does not. Cleanup must not be able to delete the chat workflow.
func TestSync_KeepsReservedWorkflowSkill(t *testing.T) {
	isolateHome(t)

	root := t.TempDir()
	reserved := filepath.Join(root, ".agents", "skills", "skmgr-find-skill")
	writeDirFile(t, reserved, "SKILL.md", "workflow")

	source := t.TempDir()
	writeSkillSource(t, source, "local-skill")

	e := newTestEngine(t, root, t.TempDir())
	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills:  []types.SkillDependency{projectSkill(source, "local-skill", types.ScopeProject)},
	}
	lock := &types.Lockfile{
		Version: types.LockfileVersion,
		Entries: []types.LockEntry{{Name: "skmgr-find-skill"}},
	}
	if _, err := e.Sync(manifest, lock, SyncOptions{}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(reserved, "SKILL.md")); err != nil {
		t.Errorf("reserved workflow skill was deleted: %v", err)
	}
}
