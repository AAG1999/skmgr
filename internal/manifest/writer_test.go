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
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// Parsing fills in derived names and default type/scope. Writing that straight
// back out would bake values into the file that the user never typed, and turn
// an unrelated `skmgr add` into a large diff.
func TestWrite_DropsDerivedNamesAndDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.yml")

	original := `version: "1"
name: demo
targets:
  - cursor
skills:
  - source: https://example.com/team/skills.git
    path: skills/deploy-helper
  - name: my-alias
    source: https://example.com/team/skills.git
    path: skills/deploy-helper
    scope: global
`

	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	parsed, err := Parse(path)
	if err != nil {
		t.Fatalf("parsing: %v", err)
	}

	if parsed.Skills[0].Name != "deploy-helper" {
		t.Fatalf("expected a derived name in memory, got %q", parsed.Skills[0].Name)
	}

	if err := Write(path, parsed); err != nil {
		t.Fatalf("writing: %v", err)
	}

	written, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	got := string(written)

	if strings.Contains(got, "name: deploy-helper") {
		t.Errorf("derived name should not be written back:\n%s", got)
	}
	if strings.Contains(got, "type: skill") {
		t.Errorf("default type should not be written back:\n%s", got)
	}
	if strings.Contains(got, "scope: project") {
		t.Errorf("default scope should not be written back:\n%s", got)
	}
	if !strings.Contains(got, "name: my-alias") {
		t.Errorf("an explicit name must survive:\n%s", got)
	}
	if !strings.Contains(got, "scope: global") {
		t.Errorf("a non-default scope must survive:\n%s", got)
	}
}

// Writing must not mutate the manifest the caller still holds.
func TestWrite_DoesNotMutateCaller(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.yml")

	m := &types.Manifest{
		Version: "1",
		Name:    "demo",
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{{
			Name:        "deploy-helper",
			NameDerived: true,
			Source:      "https://example.com/team/skills.git",
			Path:        "skills/deploy-helper",
			Type:        types.TypeSkill,
			Scope:       types.ScopeProject,
		}},
	}

	if err := Write(path, m); err != nil {
		t.Fatal(err)
	}

	if m.Skills[0].Name != "deploy-helper" {
		t.Errorf("Name was cleared on the caller's copy: %q", m.Skills[0].Name)
	}
	if m.Skills[0].Type != types.TypeSkill {
		t.Errorf("Type was cleared on the caller's copy: %q", m.Skills[0].Type)
	}
}

// A parse-write-parse cycle must be a no-op.
func TestWrite_RoundTripsStably(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "skmgr.yml")

	original := `version: "1"
name: demo
targets:
  - cursor
skills:
  - source: https://example.com/team/skills.git
    path: skills/deploy-helper
`

	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}

	first, err := Parse(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(path, first); err != nil {
		t.Fatal(err)
	}
	afterFirst, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	second, err := Parse(path)
	if err != nil {
		t.Fatalf("re-parsing what we wrote: %v", err)
	}
	if err := Write(path, second); err != nil {
		t.Fatal(err)
	}
	afterSecond, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if string(afterFirst) != string(afterSecond) {
		t.Errorf("write is not idempotent:\nfirst:\n%s\nsecond:\n%s", afterFirst, afterSecond)
	}
	if second.Skills[0].Name != "deploy-helper" {
		t.Errorf("name should still derive after a round trip, got %q", second.Skills[0].Name)
	}
}
