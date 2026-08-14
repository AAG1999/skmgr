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

// writeRuleSource creates a rule the local provider accepts.
func writeRuleSource(t *testing.T, root, name string) {
	t.Helper()
	dir := filepath.Join(root, "rules", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating rule source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# "+name), 0644); err != nil {
		t.Fatalf("writing AGENTS.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("rule body for "+name), 0644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}
}

// Merged rules live inside a shared markdown file, so there is no symlink for
// the broken-link sweep to find. Removing the rule from the manifest has to
// scrub the delimited block explicitly.
func TestSync_RemovesMergedRuleSectionWhenSkillDropped(t *testing.T) {
	isolateHome(t)

	root := t.TempDir()
	source := t.TempDir()
	writeRuleSource(t, source, "team-rule")

	e := newTestEngine(t, root, t.TempDir())

	withRule := &types.Manifest{
		Targets: []string{"claude-code"},
		Skills: []types.SkillDependency{
			{
				Name:   "team-rule",
				Source: "file://" + source,
				Path:   "rules/team-rule",
				Type:   types.TypeRule,
				Scope:  types.ScopeProject,
			},
		},
	}

	lock, err := e.Sync(withRule, nil, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync with rule failed: %v", err)
	}

	claudeMD := filepath.Join(root, ".claude", "CLAUDE.md")
	before, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("expected CLAUDE.md to be written: %v", err)
	}
	if !strings.Contains(string(before), "skmgr:start:team-rule") {
		t.Fatalf("merged section was not created; got:\n%s", before)
	}

	// Drop the rule and sync again, passing the previous lock so the engine
	// knows what disappeared.
	withoutRule := &types.Manifest{
		Targets: []string{"claude-code"},
		Skills:  nil,
	}

	if _, err := e.Sync(withoutRule, lock, SyncOptions{}); err != nil {
		t.Fatalf("Sync after removal failed: %v", err)
	}

	after, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("reading CLAUDE.md after removal: %v", err)
	}
	if strings.Contains(string(after), "skmgr:start:team-rule") {
		t.Errorf("merged rule section survived removal; got:\n%s", after)
	}
}

// Removing a managed section must not disturb the user's own content.
func TestSync_MergedRuleRemovalPreservesUserContent(t *testing.T) {
	isolateHome(t)

	root := t.TempDir()
	source := t.TempDir()
	writeRuleSource(t, source, "team-rule")

	claudeDir := filepath.Join(root, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")
	handWritten := "# My own instructions\n\nAlways run the linter.\n"
	if err := os.WriteFile(claudeMD, []byte(handWritten), 0644); err != nil {
		t.Fatal(err)
	}

	e := newTestEngine(t, root, t.TempDir())

	withRule := &types.Manifest{
		Targets: []string{"claude-code"},
		Skills: []types.SkillDependency{
			{
				Name:   "team-rule",
				Source: "file://" + source,
				Path:   "rules/team-rule",
				Type:   types.TypeRule,
				Scope:  types.ScopeProject,
			},
		},
	}

	lock, err := e.Sync(withRule, nil, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync with rule failed: %v", err)
	}

	if _, err := e.Sync(&types.Manifest{Targets: []string{"claude-code"}}, lock, SyncOptions{}); err != nil {
		t.Fatalf("Sync after removal failed: %v", err)
	}

	after, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(after), "Always run the linter.") {
		t.Errorf("user content was lost during managed section removal; got:\n%s", after)
	}
}
