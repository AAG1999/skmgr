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

func TestResolve_LocalSource(t *testing.T) {
	cache := t.TempDir()
	source := t.TempDir()

	skillsDir := filepath.Join(source, "skills", "local-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("hello"), 0644)

	r := NewResolver(cache)
	skills := []types.SkillDependency{
		{
			Name:   "local-skill",
			Source: "file://" + source,
		},
	}

	res, err := r.Resolve(skills)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(res) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(res))
	}

	if res[0].CommitSHA != "" {
		t.Error("Local sources should not have a commit SHA")
	}
}

func TestResolve_ConflictingNames(t *testing.T) {
	cache := t.TempDir()
	source := t.TempDir()

	skillsDir := filepath.Join(source, "skills", "duplicate")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte(""), 0644)

	r := NewResolver(cache)
	skills := []types.SkillDependency{
		{Name: "duplicate", Source: "file://" + source},
		{Name: "duplicate", Source: "file://" + source},
	}

	_, err := r.Resolve(skills)
	if err == nil {
		t.Fatal("Expected error for conflicting names, got nil")
	}
	if !strings.Contains(err.Error(), "duplicate skill name") {
		t.Errorf("Expected duplicate error, got: %v", err)
	}
}

func TestResolve_InvalidPath(t *testing.T) {
	cache := t.TempDir()
	source := t.TempDir()

	r := NewResolver(cache)
	skills := []types.SkillDependency{
		{
			Name:   "local-skill",
			Source: "file://" + source,
			Path:   "nonexistent",
		},
	}
	// For LocalProvider, we don't extract paths the same way in provider. Fetch just verifies `source`.
	// Let's actually test an invalid local path at the source level.
	skills[0].Source = "file://" + filepath.Join(source, "nonexistent")

	_, err := r.Resolve(skills)
	if err == nil {
		t.Fatal("Expected error for invalid path, got nil")
	}
}

func TestResolve_AutoDetectType(t *testing.T) {
	cache := t.TempDir()
	source := t.TempDir()

	// Create two directories matching a wildcard, one with SKILL.md, one with AGENTS.md
	skillDir := filepath.Join(source, "skills", "my-skill")
	ruleDir := filepath.Join(source, "rules", "my-rule")
	_ = os.MkdirAll(skillDir, 0755)
	_ = os.MkdirAll(ruleDir, 0755)

	_ = os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(""), 0644)
	_ = os.WriteFile(filepath.Join(ruleDir, "AGENTS.md"), []byte(""), 0644)

	r := NewResolver(cache)
	skills := []types.SkillDependency{
		{
			Source: "file://" + source,
			Path:   "skills/*",
		},
		{
			Source: "file://" + source,
			Path:   "rules/*",
		},
	}

	res, err := r.Resolve(skills)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(res) != 2 {
		t.Fatalf("Expected 2 results, got %d", len(res))
	}

	var foundSkill, foundRule bool
	for _, r := range res {
		if filepath.Base(r.SourceDir) == "my-skill" {
			foundSkill = true
			if r.SkillDependency.Type != types.TypeSkill {
				t.Errorf("Expected type %q for my-skill, got %q", types.TypeSkill, r.SkillDependency.Type)
			}
		} else if filepath.Base(r.SourceDir) == "my-rule" {
			foundRule = true
			if r.SkillDependency.Type != types.TypeRule {
				t.Errorf("Expected type %q for my-rule, got %q", types.TypeRule, r.SkillDependency.Type)
			}
		}
	}

	if !foundSkill || !foundRule {
		t.Errorf("Did not find both expected generated skills. foundSkill=%v, foundRule=%v", foundSkill, foundRule)
	}
}
