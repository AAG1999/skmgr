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

package types

import (
	"path/filepath"
	"testing"

	yaml "gopkg.in/yaml.v3"
)

// --- SkillDependency default tests ---

func TestSkillDependency_DefaultType(t *testing.T) {
	s := SkillDependency{Name: "test", Source: "https://github.com/user/repo.git"}

	if s.EffectiveType() != TypeSkill {
		t.Errorf("EffectiveType() = %q, want %q", s.EffectiveType(), TypeSkill)
	}

	// Explicit type should be respected.
	s.Type = TypeRule
	if s.EffectiveType() != TypeRule {
		t.Errorf("EffectiveType() = %q, want %q", s.EffectiveType(), TypeRule)
	}
}

func TestSkillDependency_DefaultScope(t *testing.T) {
	s := SkillDependency{Name: "test", Source: "https://github.com/user/repo.git"}

	if s.EffectiveScope() != ScopeProject {
		t.Errorf("EffectiveScope() = %q, want %q", s.EffectiveScope(), ScopeProject)
	}

	// Explicit scope should be respected.
	s.Scope = ScopeGlobal
	if s.EffectiveScope() != ScopeGlobal {
		t.Errorf("EffectiveScope() = %q, want %q", s.EffectiveScope(), ScopeGlobal)
	}
}

func TestSkillDependency_EffectiveTargets(t *testing.T) {
	manifestTargets := []string{"cursor", "gemini"}

	// No override: falls back to manifest targets.
	s := SkillDependency{Name: "test", Source: "https://github.com/user/repo.git"}
	got := s.EffectiveTargets(manifestTargets)
	if len(got) != 2 || got[0] != "cursor" || got[1] != "gemini" {
		t.Errorf("EffectiveTargets() = %v, want %v", got, manifestTargets)
	}

	// With override: uses skill-level targets.
	s.Targets = []string{"claude-code"}
	got = s.EffectiveTargets(manifestTargets)
	if len(got) != 1 || got[0] != "claude-code" {
		t.Errorf("EffectiveTargets() = %v, want %v", got, []string{"claude-code"})
	}
}

// --- YAML round-trip tests ---

func TestManifest_YAMLRoundTrip(t *testing.T) {
	original := Manifest{
		Name:    "my-project",
		Version: "1.0",
		Targets: []string{"cursor", "gemini"},
		Skills: []SkillDependency{
			{
				Name:   "frontend-design",
				Source: "https://github.com/anthropics/skills.git",
				Path:   "skills/frontend-design",
				Ref:    "v2.1.0",
			},
			{
				Name:   "coding-standards",
				Source: "https://github.com/acme/standards.git",
				Path:   "rules/typescript",
				Type:   TypeRule,
				Ref:    "v1.0.0",
				Scope:  ScopeGlobal,
			},
			{
				Name:    "deploy-helper",
				Source:  "https://gitlab.internal.com/team/agent-skills.git",
				Path:    "deploy-helper",
				Ref:     "v3.0",
				Targets: []string{"claude-code"},
			},
		},
	}

	// Marshal to YAML.
	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal() error: %v", err)
	}

	// Unmarshal back.
	var restored Manifest
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("yaml.Unmarshal() error: %v", err)
	}

	// Compare fields.
	if restored.Name != original.Name {
		t.Errorf("Name = %q, want %q", restored.Name, original.Name)
	}
	if restored.Version != original.Version {
		t.Errorf("Version = %q, want %q", restored.Version, original.Version)
	}
	if len(restored.Targets) != len(original.Targets) {
		t.Fatalf("Targets length = %d, want %d", len(restored.Targets), len(original.Targets))
	}
	for i, target := range restored.Targets {
		if target != original.Targets[i] {
			t.Errorf("Targets[%d] = %q, want %q", i, target, original.Targets[i])
		}
	}
	if len(restored.Skills) != len(original.Skills) {
		t.Fatalf("Skills length = %d, want %d", len(restored.Skills), len(original.Skills))
	}
	for i, skill := range restored.Skills {
		orig := original.Skills[i]
		if skill.Name != orig.Name {
			t.Errorf("Skills[%d].Name = %q, want %q", i, skill.Name, orig.Name)
		}
		if skill.Source != orig.Source {
			t.Errorf("Skills[%d].Source = %q, want %q", i, skill.Source, orig.Source)
		}
		if skill.Path != orig.Path {
			t.Errorf("Skills[%d].Path = %q, want %q", i, skill.Path, orig.Path)
		}
		if skill.Ref != orig.Ref {
			t.Errorf("Skills[%d].Ref = %q, want %q", i, skill.Ref, orig.Ref)
		}
		if skill.Type != orig.Type {
			t.Errorf("Skills[%d].Type = %q, want %q", i, skill.Type, orig.Type)
		}
		if skill.Scope != orig.Scope {
			t.Errorf("Skills[%d].Scope = %q, want %q", i, skill.Scope, orig.Scope)
		}
		if len(skill.Targets) != len(orig.Targets) {
			t.Errorf("Skills[%d].Targets length = %d, want %d", i, len(skill.Targets), len(orig.Targets))
		}
	}
}

func TestLockEntry_YAMLRoundTrip(t *testing.T) {
	original := Lockfile{
		Version:     LockfileVersion,
		GeneratedAt: "2026-07-15T10:00:00Z",
		Entries: []LockEntry{
			{
				Name:        "frontend-design",
				Source:      "https://github.com/anthropics/skills.git",
				Path:        "skills/frontend-design",
				CommitSHA:   "abc123def456",
				ContentHash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
				ResolvedAt:  "2026-07-15T10:00:00Z",
			},
			{
				Name:        "my-local-skill",
				Source:      "file:///Users/me/skills/custom",
				ContentHash: "sha256:deadbeef",
				ResolvedAt:  "2026-07-15T10:00:00Z",
			},
		},
	}

	// Marshal to YAML.
	data, err := yaml.Marshal(&original)
	if err != nil {
		t.Fatalf("yaml.Marshal() error: %v", err)
	}

	// Unmarshal back.
	var restored Lockfile
	if err := yaml.Unmarshal(data, &restored); err != nil {
		t.Fatalf("yaml.Unmarshal() error: %v", err)
	}

	// Compare.
	if restored.Version != original.Version {
		t.Errorf("Version = %q, want %q", restored.Version, original.Version)
	}
	if restored.GeneratedAt != original.GeneratedAt {
		t.Errorf("GeneratedAt = %q, want %q", restored.GeneratedAt, original.GeneratedAt)
	}
	if len(restored.Entries) != len(original.Entries) {
		t.Fatalf("Entries length = %d, want %d", len(restored.Entries), len(original.Entries))
	}
	for i, entry := range restored.Entries {
		orig := original.Entries[i]
		if entry.Name != orig.Name {
			t.Errorf("Entries[%d].Name = %q, want %q", i, entry.Name, orig.Name)
		}
		if entry.Source != orig.Source {
			t.Errorf("Entries[%d].Source = %q, want %q", i, entry.Source, orig.Source)
		}
		if entry.Path != orig.Path {
			t.Errorf("Entries[%d].Path = %q, want %q", i, entry.Path, orig.Path)
		}
		if entry.CommitSHA != orig.CommitSHA {
			t.Errorf("Entries[%d].CommitSHA = %q, want %q", i, entry.CommitSHA, orig.CommitSHA)
		}
		if entry.ContentHash != orig.ContentHash {
			t.Errorf("Entries[%d].ContentHash = %q, want %q", i, entry.ContentHash, orig.ContentHash)
		}
	}
}

// --- Lockfile helper tests ---

func TestLockfile_FindEntry(t *testing.T) {
	lf := &Lockfile{
		Version: LockfileVersion,
		Entries: []LockEntry{
			{Name: "skill-a", Source: "https://github.com/a.git"},
			{Name: "skill-b", Source: "https://github.com/b.git"},
		},
	}

	// Found.
	entry := lf.FindEntry("skill-a")
	if entry == nil {
		t.Fatal("FindEntry(\"skill-a\") returned nil, want non-nil")
	}
	if entry.Source != "https://github.com/a.git" {
		t.Errorf("FindEntry(\"skill-a\").Source = %q, want %q", entry.Source, "https://github.com/a.git")
	}

	// Not found.
	if lf.FindEntry("nonexistent") != nil {
		t.Error("FindEntry(\"nonexistent\") returned non-nil, want nil")
	}
}

func TestLockfile_SetEntry(t *testing.T) {
	lf := &Lockfile{
		Version: LockfileVersion,
		Entries: []LockEntry{
			{Name: "skill-a", Source: "https://github.com/a.git", CommitSHA: "old"},
		},
	}

	// Update existing.
	lf.SetEntry(LockEntry{Name: "skill-a", Source: "https://github.com/a.git", CommitSHA: "new"})
	if len(lf.Entries) != 1 {
		t.Fatalf("Entries length = %d, want 1 (should replace, not append)", len(lf.Entries))
	}
	if lf.Entries[0].CommitSHA != "new" {
		t.Errorf("CommitSHA = %q, want %q", lf.Entries[0].CommitSHA, "new")
	}

	// Add new.
	lf.SetEntry(LockEntry{Name: "skill-b", Source: "https://github.com/b.git"})
	if len(lf.Entries) != 2 {
		t.Fatalf("Entries length = %d, want 2", len(lf.Entries))
	}
}

func TestLockfile_RemoveEntry(t *testing.T) {
	lf := &Lockfile{
		Version: LockfileVersion,
		Entries: []LockEntry{
			{Name: "skill-a"},
			{Name: "skill-b"},
			{Name: "skill-c"},
		},
	}

	// Remove middle entry.
	removed := lf.RemoveEntry("skill-b")
	if !removed {
		t.Error("RemoveEntry(\"skill-b\") returned false, want true")
	}
	if len(lf.Entries) != 2 {
		t.Fatalf("Entries length = %d, want 2", len(lf.Entries))
	}
	if lf.FindEntry("skill-b") != nil {
		t.Error("skill-b still present after removal")
	}

	// Remove non-existent.
	removed = lf.RemoveEntry("nonexistent")
	if removed {
		t.Error("RemoveEntry(\"nonexistent\") returned true, want false")
	}
}

// --- Agent definition tests ---

func TestDefaultAgents_AllRegistered(t *testing.T) {
	agents := DefaultAgents()
	expected := SupportedAgentNames()

	for _, name := range expected {
		if _, ok := agents[name]; !ok {
			t.Errorf("DefaultAgents() missing agent %q", name)
		}
	}
}

func TestCursorAgent_SkillsDir(t *testing.T) {
	agent := DefaultAgents()["cursor"]

	projectDir := filepath.ToSlash(agent.SkillsDir(ScopeProject, "/my/project"))
	if projectDir != "/my/project/.cursor/skills" {
		t.Errorf("SkillsDir(project) = %q, want %q", projectDir, "/my/project/.cursor/skills")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Global scope ignores the project root and uses the home directory.
	wantGlobal := filepath.ToSlash(filepath.Join(home, ".cursor", "skills"))
	globalDir := filepath.ToSlash(agent.SkillsDir(ScopeGlobal, "/my/project"))
	if globalDir != wantGlobal {
		t.Errorf("SkillsDir(global) = %q, want %q", globalDir, wantGlobal)
	}
}

func TestGeminiAgent_ReadsFromAgents(t *testing.T) {
	agent := DefaultAgents()["gemini"]

	if !agent.ReadsFromAgents(ScopeProject) {
		t.Error("Gemini ReadsFromAgents(project) = false, want true")
	}
	if agent.ReadsFromAgents(ScopeGlobal) {
		t.Error("Gemini ReadsFromAgents(global) = true, want false")
	}
}

func TestGeminiAgent_SkillsDir(t *testing.T) {
	agent := DefaultAgents()["gemini"]

	// Project scope: returns .agents/skills/ (native read).
	projectDir := filepath.ToSlash(agent.SkillsDir(ScopeProject, "/my/project"))
	if projectDir != "/my/project/.agents/skills" {
		t.Errorf("SkillsDir(project) = %q, want %q", projectDir, "/my/project/.agents/skills")
	}

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Global scope: returns .gemini/config/skills/ under the home directory.
	wantGlobal := filepath.ToSlash(filepath.Join(home, ".gemini", "config", "skills"))
	globalDir := filepath.ToSlash(agent.SkillsDir(ScopeGlobal, "/my/project"))
	if globalDir != wantGlobal {
		t.Errorf("SkillsDir(global) = %q, want %q", globalDir, wantGlobal)
	}
}

func TestGeminiAgent_RuleStrategy(t *testing.T) {
	agent := DefaultAgents()["gemini"]

	if agent.RuleStrategyForScope(ScopeProject) != RuleStrategySkip {
		t.Errorf("RuleStrategy(project) = %q, want %q", agent.RuleStrategyForScope(ScopeProject), RuleStrategySkip)
	}
	if agent.RuleStrategyForScope(ScopeGlobal) != RuleStrategyMerge {
		t.Errorf("RuleStrategy(global) = %q, want %q", agent.RuleStrategyForScope(ScopeGlobal), RuleStrategyMerge)
	}
}

func TestClaudeCodeAgent_RuleStrategy(t *testing.T) {
	agent := DefaultAgents()["claude-code"]

	if agent.RuleStrategyForScope(ScopeProject) != RuleStrategyMerge {
		t.Errorf("RuleStrategy(project) = %q, want %q", agent.RuleStrategyForScope(ScopeProject), RuleStrategyMerge)
	}
}

func TestCopilotAgent_Paths(t *testing.T) {
	agent := DefaultAgents()["copilot"]

	skillsDir := filepath.ToSlash(agent.SkillsDir(ScopeProject, "/my/project"))
	if skillsDir != "/my/project/.github/skills" {
		t.Errorf("SkillsDir(project) = %q, want %q", skillsDir, "/my/project/.github/skills")
	}

	rulesPath := filepath.ToSlash(agent.RulesPath(ScopeProject, "/my/project"))
	if rulesPath != "/my/project/.github/copilot-instructions.md" {
		t.Errorf("RulesPath(project) = %q, want %q", rulesPath, "/my/project/.github/copilot-instructions.md")
	}
}

// --- Canonical path tests ---

func TestCanonicalSkillsDir_Project(t *testing.T) {
	got := filepath.ToSlash(CanonicalSkillsDir(ScopeProject, "/my/project"))
	want := "/my/project/.agents/skills"
	if got != want {
		t.Errorf("CanonicalSkillsDir(project) = %q, want %q", got, want)
	}
}

func TestCanonicalRulesDir_Project(t *testing.T) {
	got := filepath.ToSlash(CanonicalRulesDir(ScopeProject, "/my/project"))
	want := "/my/project/.agents/rules"
	if got != want {
		t.Errorf("CanonicalRulesDir(project) = %q, want %q", got, want)
	}
}
