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
	"strings"
	"testing"
)

// runList executes list and returns whatever it wrote to stdout.
func runList(t *testing.T) string {
	t.Helper()

	var out bytes.Buffer
	listCmd.SetOut(&out)
	t.Cleanup(func() { listCmd.SetOut(nil) })

	if err := listCmd.RunE(listCmd, nil); err != nil {
		t.Fatalf("list failed: %v", err)
	}
	return out.String()
}

func TestList_NoManifest(t *testing.T) {
	newProjectDir(t)

	if err := listCmd.RunE(listCmd, nil); err == nil {
		t.Fatal("list without a manifest should fail")
	}
}

func TestList_ShowsMissingBeforeInstall(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "my-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/my-skill
`, source))

	out := runList(t)

	if !strings.Contains(out, "my-skill") {
		t.Errorf("list output does not mention the skill:\n%s", out)
	}
	if !strings.Contains(out, statusMissing) {
		t.Errorf("list should report the skill as missing before install:\n%s", out)
	}
}

func TestList_ShowsInstalledAfterInstall(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "my-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/my-skill
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	out := runList(t)
	if !strings.Contains(out, statusInstalled) {
		t.Errorf("list should report the skill as installed:\n%s", out)
	}
}

// The old --json output was hand-built with %s and broke on any character
// needing escaping. It must be real JSON now.
func TestList_JSONIsValidAndComplete(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "my-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
  - claude-code
skills:
  - source: file://%s
    path: skills/my-skill
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	listJSON = true
	out := runList(t)

	var rows []skillStatus
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("list --json produced invalid JSON: %v\n%s", err, out)
	}

	if len(rows) != 1 {
		t.Fatalf("got %d rows, want 1", len(rows))
	}

	row := rows[0]
	if row.Name != "my-skill" {
		t.Errorf("Name = %q, want %q", row.Name, "my-skill")
	}
	if row.Status != statusInstalled {
		t.Errorf("Status = %q, want %q", row.Status, statusInstalled)
	}
	if len(row.Targets) != 2 {
		t.Errorf("Targets = %v, want both manifest targets", row.Targets)
	}
	if row.Source == "" {
		t.Error("JSON output is missing the source")
	}
}

// The old output was assembled with %s, so any value needing escaping produced
// malformed JSON. Source and path come from user data and can contain such
// characters even though names cannot.
func TestList_JSONEscapesSpecialCharacters(t *testing.T) {
	rows := []skillStatus{{
		Name:    "quirky",
		Type:    "skill",
		Scope:   "project",
		Source:  `https://example.com/repo.git?q="x"&y=1`,
		Path:    `skills/with"quote`,
		Status:  statusInstalled,
		Targets: []string{"cursor"},
	}}

	encoded, err := json.Marshal(rows)
	if err != nil {
		t.Fatalf("marshalling status rows: %v", err)
	}

	var decoded []skillStatus
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, encoded)
	}

	if decoded[0].Source != rows[0].Source {
		t.Errorf("Source = %q, want %q", decoded[0].Source, rows[0].Source)
	}
	if decoded[0].Path != rows[0].Path {
		t.Errorf("Path = %q, want %q", decoded[0].Path, rows[0].Path)
	}
}

// A wildcard entry has no name in the manifest; list should show what the
// lockfile records was actually installed.
func TestList_ExpandsWildcardFromLockfile(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "alpha", "beta")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/*
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	listJSON = true
	out := runList(t)

	var rows []skillStatus
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}

	names := make(map[string]bool)
	for _, r := range rows {
		names[r.Name] = true
	}

	for _, want := range []string{"alpha", "beta"} {
		if !names[want] {
			t.Errorf("list output is missing wildcard member %q; got %v", want, names)
		}
	}
}
