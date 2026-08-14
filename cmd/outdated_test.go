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

func runOutdated(t *testing.T) string {
	t.Helper()

	var out bytes.Buffer
	outdatedCmd.SetOut(&out)
	t.Cleanup(func() { outdatedCmd.SetOut(nil) })

	if err := outdatedCmd.RunE(outdatedCmd, nil); err != nil {
		t.Fatalf("outdated: %v", err)
	}

	return out.String()
}

func TestOutdated_ReportsCurrentWantedAndLatest(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0", "v1.1.0", "v2.0.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	// Publish a patch inside the range, so wanted moves ahead of current while
	// latest stays outside it.
	tagSourceVersion(t, strings.TrimPrefix(source, "git::"), "tool", "v1.2.0")

	outdatedJSON = true
	out := runOutdated(t)

	var rows []outdatedRow
	if err := json.Unmarshal([]byte(out), &rows); err != nil {
		t.Fatalf("outdated --json is not valid JSON: %v\n%s", err, out)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}

	row := rows[0]
	if row.Range != "^1.0.0" {
		t.Errorf("Range = %q, want ^1.0.0", row.Range)
	}
	if row.CurrentVersion != "v1.1.0" {
		t.Errorf("Current = %q, want v1.1.0", row.CurrentVersion)
	}
	if row.WantedVersion != "v1.2.0" {
		t.Errorf("Wanted = %q, want v1.2.0: the newest inside the range", row.WantedVersion)
	}
	if row.LatestVersion != "v2.0.0" {
		t.Errorf("Latest = %q, want v2.0.0: the newest that exists", row.LatestVersion)
	}
	if !row.Outdated() {
		t.Error("a row whose wanted version is ahead of current is outdated")
	}
}

func TestOutdated_UpToDateWithinTheRange(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0", "v1.1.0", "v2.0.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	logs := captureLog(t)
	out := runOutdated(t)

	if !strings.Contains(logs.String(), "Up to date") {
		t.Errorf("expected an up-to-date report, got:\n%s", logs.String())
	}

	// A newer major exists, and saying so is the point: the user cannot see it
	// from the table's "wanted" column alone.
	if !strings.Contains(logs.String(), "v2.0.0") {
		t.Errorf("expected the out-of-range version to be mentioned, got:\n%s", logs.String())
	}
	if !strings.Contains(out, "v2.0.0") {
		t.Errorf("expected v2.0.0 in the latest column, got:\n%s", out)
	}
}

// A branch or tag ref has no range to compare against, and saying nothing at
// all would look like the skill was simply missed.
func TestOutdated_NonRangedSkillIsExplained(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0")
	rangeManifest(t, source, "tool", "v1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	outdatedJSON = true
	var rows []outdatedRow
	if err := json.Unmarshal([]byte(runOutdated(t)), &rows); err != nil {
		t.Fatal(err)
	}

	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].Note != "not version-ranged" {
		t.Errorf("Note = %q, want an explanation", rows[0].Note)
	}
	if rows[0].Outdated() {
		t.Error("a skill with no range cannot be outdated")
	}
}

func TestOutdated_TableIncludesEveryColumn(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0", "v2.0.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	out := runOutdated(t)

	for _, want := range []string{"NAME", "RANGE", "CURRENT", "WANTED", "LATEST", "tool"} {
		if !strings.Contains(out, want) {
			t.Errorf("table should contain %q, got:\n%s", want, out)
		}
	}
}

func TestOutdated_NoSkillsDeclared(t *testing.T) {
	newProjectDir(t)
	writeManifest(t, "skmgr.yml", `version: "1"
name: empty
targets:
  - cursor
skills: []
`)

	logs := captureLog(t)
	if out := runOutdated(t); strings.Contains(out, "NAME") {
		t.Errorf("an empty project should not print a table, got:\n%s", out)
	}
	if !strings.Contains(logs.String(), "No skills declared") {
		t.Errorf("expected an explanation, got:\n%s", logs.String())
	}
}

// Before an install there is no lockfile, so there is no current version. The
// command still has to report what is available rather than failing.
func TestOutdated_WithoutALockfile(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0", "v1.1.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	outdatedJSON = true
	var rows []outdatedRow
	if err := json.Unmarshal([]byte(runOutdated(t)), &rows); err != nil {
		t.Fatal(err)
	}

	if rows[0].CurrentVersion != "" {
		t.Errorf("Current = %q, want empty before any install", rows[0].CurrentVersion)
	}
	if rows[0].WantedVersion != "v1.1.0" {
		t.Errorf("Wanted = %q, want v1.1.0", rows[0].WantedVersion)
	}
}

// A repository with no version tags cannot answer the question, and the row
// has to say why instead of showing blanks.
func TestOutdated_UntaggedSourceIsExplained(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool")

	// A repository with no tags still needs a commit to be clonable.
	tagless := strings.TrimPrefix(source, "git::")
	gitInSource(t, tagless, "add", ".")
	gitInSource(t, tagless, "commit", "--quiet", "--allow-empty", "-m", "initial")

	rangeManifest(t, source, "tool", "^1.0.0")

	outdatedJSON = true
	var rows []outdatedRow
	if err := json.Unmarshal([]byte(runOutdated(t)), &rows); err != nil {
		t.Fatal(err)
	}

	if rows[0].WantedVersion != "" {
		t.Errorf("Wanted = %q, want empty when there are no versions", rows[0].WantedVersion)
	}
	if !strings.Contains(rows[0].Note, "no tags") {
		t.Errorf("Note = %q, want it to explain the missing tags", rows[0].Note)
	}
}

// One repository backing several skills must be queried once, not once per
// skill: tag lookup can hit the network.
func TestOutdated_SharesTagLookupsAcrossSkills(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "alpha", "v1.0.0", "v1.1.0")

	// Add a second skill to the same repository.
	root := strings.TrimPrefix(source, "git::")
	tagSourceVersion(t, root, "beta", "v1.2.0")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: shared
targets:
  - cursor
skills:
  - source: %s
    path: skills/alpha
    ref: "^1.0.0"
  - source: %s
    path: skills/beta
    ref: "^1.0.0"
`, source, source))

	outdatedJSON = true
	var rows []outdatedRow
	if err := json.Unmarshal([]byte(runOutdated(t)), &rows); err != nil {
		t.Fatal(err)
	}

	if len(rows) != 2 {
		t.Fatalf("expected two rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.WantedVersion != "v1.2.0" {
			t.Errorf("%s wanted = %q, want v1.2.0 from the shared repository",
				row.Name, row.WantedVersion)
		}
	}
}
