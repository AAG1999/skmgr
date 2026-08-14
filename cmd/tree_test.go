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

package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// runTree executes the command and captures its output.
func runTree(t *testing.T) (string, error) {
	t.Helper()

	var out bytes.Buffer
	treeCmd.SetOut(&out)
	t.Cleanup(func() { treeCmd.SetOut(nil) })

	err := treeCmd.RunE(treeCmd, nil)
	return out.String(), err
}

func TestTree_ShowsScopesSkillsAndTargets(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "greeter", "reviewer")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
  - claude-code
skills:
  - source: file://%s
    path: skills/greeter
  - source: file://%s
    path: skills/reviewer
`, source, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	output, err := runTree(t)
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}

	for _, want := range []string{"project", "greeter", "reviewer", "cursor", "claude-code", "installed"} {
		if !strings.Contains(output, want) {
			t.Errorf("the layout should mention %q, got:\n%s", want, output)
		}
	}
}

// The point of tree over list is spotting a skill that is not where it should
// be, so the status has to be honest about what is on disk.
func TestTree_MarksAMissingSkill(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	if err := os.RemoveAll(filepath.Join(dir, ".agents", "skills", "greeter")); err != nil {
		t.Fatal(err)
	}

	output, err := runTree(t)
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}

	if !strings.Contains(output, statusMissing) {
		t.Errorf("a deleted skill should show as %q, got:\n%s", statusMissing, output)
	}
}

func TestTree_JSONGroupsByScope(t *testing.T) {
	installOneSkill(t, "greeter")

	treeJSON = true
	t.Cleanup(func() { treeJSON = false })

	output, err := runTree(t)
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}

	var scopes []treeScope
	if err := json.Unmarshal([]byte(output), &scopes); err != nil {
		t.Fatalf("decoding the layout: %v\n%s", err, output)
	}

	if len(scopes) != 1 {
		t.Fatalf("expected one scope, got %d", len(scopes))
	}
	if scopes[0].Scope != types.ScopeProject {
		t.Errorf("scope = %q, want %q", scopes[0].Scope, types.ScopeProject)
	}
	if len(scopes[0].Skills) != 1 || scopes[0].Skills[0].Name != "greeter" {
		t.Errorf("skills = %+v, want just greeter", scopes[0].Skills)
	}
	if len(scopes[0].Skills[0].Targets) == 0 {
		t.Error("a skill should carry the agents it is linked into")
	}
}

func TestTree_EmptyProject(t *testing.T) {
	newProjectDir(t)
	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills: []
`)

	output, err := runTree(t)
	if err != nil {
		t.Fatalf("tree failed: %v", err)
	}

	if !strings.Contains(output, "No skills declared") {
		t.Errorf("an empty project should say so, got:\n%s", output)
	}
}

// Project scope is listed first because it is what the reader is nearly always
// looking for.
func TestGroupByScope_OrdersProjectFirst(t *testing.T) {
	scopes := groupByScope([]skillStatus{
		{Name: "global-one", Scope: types.ScopeGlobal},
		{Name: "project-one", Scope: types.ScopeProject},
	})

	if len(scopes) != 2 {
		t.Fatalf("expected both scopes, got %d", len(scopes))
	}
	if scopes[0].Scope != types.ScopeProject {
		t.Errorf("first scope = %q, want %q", scopes[0].Scope, types.ScopeProject)
	}
}

func TestGroupByScope_SortsSkillsByName(t *testing.T) {
	scopes := groupByScope([]skillStatus{
		{Name: "zeta", Scope: types.ScopeProject},
		{Name: "alpha", Scope: types.ScopeProject},
	})

	if scopes[0].Skills[0].Name != "alpha" {
		t.Errorf("skills should be sorted, got %+v", scopes[0].Skills)
	}
}
