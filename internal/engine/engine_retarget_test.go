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
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// retargetSource builds a source directory holding one skill that is valid
// either as a skill or as a rule.
func retargetSource(t *testing.T, name string) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: " + name + "\n---\n\nBe helpful.\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return root
}

func syncOnce(t *testing.T, e *Engine, m *types.Manifest, prev *types.Lockfile) *types.Lockfile {
	t.Helper()
	lock, err := e.Sync(m, prev, SyncOptions{})
	if err != nil {
		t.Fatalf("sync failed: %v", err)
	}
	return lock
}

// Orphan cleanup keys on name alone, so flipping type used to leave the old
// copy sitting under the other canonical tree, still visible to agents.
func TestSync_TypeFlipRemovesStaleCanonicalSibling(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := retargetSource(t, "flipper")
	e := NewEngine(project, filepath.Join(home, "cache"))

	asSkill := &types.Manifest{
		Version: "1",
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{{
			Name:   "flipper",
			Source: "file://" + source,
			Path:   "flipper",
			Type:   types.TypeSkill,
		}},
	}

	lock := syncOnce(t, e, asSkill, nil)

	skillPath := filepath.Join(project, ".agents", "skills", "flipper")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("expected skill install: %v", err)
	}

	asRule := &types.Manifest{
		Version: "1",
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{{
			Name:   "flipper",
			Source: "file://" + source,
			Path:   "flipper",
			Type:   types.TypeRule,
		}},
	}

	syncOnce(t, e, asRule, lock)

	if _, err := os.Stat(skillPath); !os.IsNotExist(err) {
		t.Error("the old skill copy should be gone after flipping to a rule")
	}
	if _, err := os.Stat(filepath.Join(project, ".agents", "rules", "flipper")); err != nil {
		t.Errorf("expected the rule copy to exist: %v", err)
	}
}

// Dropping an agent from a skill's own targets left its link behind, because
// cleanup only swept the manifest-level target list.
func TestSync_ShrinkingSkillTargetsRemovesStaleLink(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := retargetSource(t, "narrowing")
	e := NewEngine(project, filepath.Join(home, "cache"))

	// The manifest-level list deliberately omits claude-code: the old cleanup
	// swept only those targets, so a per-skill override could never be undone.
	both := &types.Manifest{
		Version: "1",
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{{
			Name:    "narrowing",
			Source:  "file://" + source,
			Path:    "narrowing",
			Type:    types.TypeSkill,
			Targets: []string{"cursor", "claude-code"},
		}},
	}

	lock := syncOnce(t, e, both, nil)

	claudeLink := filepath.Join(project, ".claude", "skills", "narrowing")
	if _, err := os.Lstat(claudeLink); err != nil {
		t.Fatalf("expected a claude-code link first: %v", err)
	}

	narrowed := &types.Manifest{
		Version: "1",
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{{
			Name:    "narrowing",
			Source:  "file://" + source,
			Path:    "narrowing",
			Type:    types.TypeSkill,
			Targets: []string{"cursor"},
		}},
	}

	syncOnce(t, e, narrowed, lock)

	if _, err := os.Lstat(claudeLink); !os.IsNotExist(err) {
		t.Error("dropping claude-code from targets should remove its link")
	}
	if _, err := os.Lstat(filepath.Join(project, ".cursor", "skills", "narrowing")); err != nil {
		t.Errorf("the remaining target must keep its link: %v", err)
	}
}

// The merge case has no symlink to sweep, so a shrunk target list can only be
// cleaned by rewriting the shared file.
func TestSync_ShrinkingRuleTargetsScrubsMergedBlock(t *testing.T) {
	project := t.TempDir()
	home := t.TempDir()
	t.Setenv("HOME", home)

	source := retargetSource(t, "guidance")
	e := NewEngine(project, filepath.Join(home, "cache"))

	both := &types.Manifest{
		Version: "1",
		Targets: []string{"claude-code"},
		Skills: []types.SkillDependency{{
			Name:    "guidance",
			Source:  "file://" + source,
			Path:    "guidance",
			Type:    types.TypeRule,
			Targets: []string{"claude-code", "copilot"},
		}},
	}

	lock := syncOnce(t, e, both, nil)

	copilotFile := filepath.Join(project, ".github", "copilot-instructions.md")
	before, err := os.ReadFile(copilotFile)
	if err != nil {
		t.Fatalf("expected copilot instructions to be written: %v", err)
	}
	if !strings.Contains(string(before), "skmgr:start:guidance") {
		t.Fatalf("expected a managed block first:\n%s", before)
	}

	narrowed := &types.Manifest{
		Version: "1",
		Targets: []string{"claude-code"},
		Skills: []types.SkillDependency{{
			Name:    "guidance",
			Source:  "file://" + source,
			Path:    "guidance",
			Type:    types.TypeRule,
			Targets: []string{"claude-code"},
		}},
	}

	syncOnce(t, e, narrowed, lock)

	after, err := os.ReadFile(copilotFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(after), "skmgr:start:guidance") {
		t.Errorf("copilot block should be gone after dropping the target:\n%s", after)
	}

	claudeFile := filepath.Join(project, ".claude", "CLAUDE.md")
	kept, err := os.ReadFile(claudeFile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(kept), "skmgr:start:guidance") {
		t.Errorf("the retained target must keep its block:\n%s", kept)
	}
}
