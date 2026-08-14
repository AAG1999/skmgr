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

package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParse_ValidFull(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "manifests", "valid_full.yml")
	m, err := Parse(path)
	if err != nil {
		t.Fatalf("Parse() returned unexpected error: %v", err)
	}

	if m.Name != "test-project" {
		t.Errorf("Name = %q, want %q", m.Name, "test-project")
	}
	if len(m.Targets) != 2 || m.Targets[0] != "cursor" || m.Targets[1] != "gemini" {
		t.Errorf("Targets = %v, want [cursor, gemini]", m.Targets)
	}
	if len(m.Skills) != 3 {
		t.Fatalf("Skills length = %d, want 3", len(m.Skills))
	}

	// Spot check a skill
	s := m.Skills[1]
	if s.Name != "coding-standards" {
		t.Errorf("Skill[1].Name = %q", s.Name)
	}
	if s.Type != "rule" {
		t.Errorf("Skill[1].Type = %q", s.Type)
	}
	if s.Scope != "global" {
		t.Errorf("Skill[1].Scope = %q", s.Scope)
	}
}

func TestParse_InvalidMissingName(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "manifests", "invalid_missing_name.yml")
	_, err := Parse(path)
	if err == nil {
		t.Fatal("Parse() expected error for missing manifest name, got nil")
	}
}

func TestParse_InvalidDuplicateSkill(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "manifests", "invalid_duplicate_skill.yml")
	_, err := Parse(path)
	if err == nil {
		t.Fatal("Parse() expected error for duplicate skill, got nil")
	}
}

func TestParse_InvalidMissingSource(t *testing.T) {
	path := filepath.Join("..", "..", "testdata", "manifests", "invalid_missing_source.yml")
	_, err := Parse(path)
	if err == nil {
		t.Fatal("Parse() expected error for missing source, got nil")
	}
}

func TestParseBytes_Empty(t *testing.T) {
	_, err := ParseBytes([]byte("   \n  "))
	if err == nil {
		t.Fatal("ParseBytes() expected error for empty input, got nil")
	}
}

func TestParseBytes_InvalidType(t *testing.T) {
	yaml := `
name: test
skills:
  - name: s1
    source: src
    type: invalid_type
`
	_, err := ParseBytes([]byte(yaml))
	if err == nil {
		t.Fatal("ParseBytes() expected error for invalid type, got nil")
	}
}

func TestParseBytes_InvalidScope(t *testing.T) {
	yaml := `
name: test
skills:
  - name: s1
    source: src
    scope: invalid_scope
`
	_, err := ParseBytes([]byte(yaml))
	if err == nil {
		t.Fatal("ParseBytes() expected error for invalid scope, got nil")
	}
}

func TestFindManifestPath(t *testing.T) {
	// Create a temporary directory
	dir := t.TempDir()

	// Should fail initially
	_, err := FindManifestPath(dir)
	if err == nil {
		t.Fatal("FindManifestPath() expected error for empty dir, got nil")
	}

	// Create skmgr.yaml
	yamlPath := filepath.Join(dir, "skmgr.yaml")
	if err := os.WriteFile(yamlPath, []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	found, err := FindManifestPath(dir)
	if err != nil {
		t.Fatalf("FindManifestPath() returned error: %v", err)
	}
	if found != yamlPath {
		t.Errorf("FindManifestPath() = %q, want %q", found, yamlPath)
	}

	// Create skmgr.yml - it should take precedence
	ymlPath := filepath.Join(dir, "skmgr.yml")
	if err := os.WriteFile(ymlPath, []byte("name: test\n"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	found, err = FindManifestPath(dir)
	if err != nil {
		t.Fatalf("FindManifestPath() returned error: %v", err)
	}
	if found != ymlPath {
		t.Errorf("FindManifestPath() = %q, want %q", found, ymlPath)
	}
}
