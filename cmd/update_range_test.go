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
	"fmt"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/lockfile"
)

// rangeManifest writes a manifest declaring one skill at a version range.
func rangeManifest(t *testing.T, source string, skill string, ref string) {
	t.Helper()

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: ranged
targets:
  - cursor
skills:
  - source: %s
    path: skills/%s
    ref: %q
`, source, skill, ref))
}

func TestUpdate_StaysInsideTheDeclaredRange(t *testing.T) {
	dir := newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0", "v1.1.0", "v2.0.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	// v2.0.0 exists and is newer, but the range says v1.
	if got := installedSkillVersion(t, dir, "tool"); got != "v1.1.0" {
		t.Fatalf("installed %q, want v1.1.0: the range excludes v2", got)
	}

	// A new patch inside the range must be picked up.
	tagSourceVersion(t, strings.TrimPrefix(source, "git::"), "tool", "v1.2.0")

	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got := installedSkillVersion(t, dir, "tool"); got != "v1.2.0" {
		t.Errorf("installed %q, want v1.2.0 after update", got)
	}

	lock, err := lockfile.Read(LockfileName)
	if err != nil {
		t.Fatal(err)
	}

	entry := lock.FindEntry("tool")
	if entry == nil {
		t.Fatal("no lock entry for tool")
	}
	if entry.Ref != "^1.0.0" {
		t.Errorf("Ref = %q, want the declared range preserved", entry.Ref)
	}
	if entry.ResolvedRef != "v1.2.0" {
		t.Errorf("ResolvedRef = %q, want v1.2.0", entry.ResolvedRef)
	}
}

func TestUpdate_LatestCrossesTheRange(t *testing.T) {
	dir := newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0", "v1.1.0", "v2.0.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}
	if got := installedSkillVersion(t, dir, "tool"); got != "v1.1.0" {
		t.Fatalf("installed %q, want v1.1.0", got)
	}

	updateLatest = true
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update --latest: %v", err)
	}

	if got := installedSkillVersion(t, dir, "tool"); got != "v2.0.0" {
		t.Errorf("installed %q, want v2.0.0 with --latest", got)
	}

	// The manifest is not rewritten, so the range still governs the next run.
	// Making --latest sticky would silently widen a constraint the team agreed
	// on, and nothing in the manifest would show it had happened.
	updateLatest = false
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := installedSkillVersion(t, dir, "tool"); got != "v1.1.0" {
		t.Errorf("installed %q, want v1.1.0: a plain update returns to the range", got)
	}
}

// A named update must not let --latest drag the other skills across a major.
func TestUpdate_LatestOnlyMovesTheNamedSkill(t *testing.T) {
	dir := newProjectDir(t)

	first := newTaggedSource(t, "alpha", "v1.0.0", "v2.0.0")
	second := newTaggedSource(t, "beta", "v1.0.0", "v2.0.0")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: ranged
targets:
  - cursor
skills:
  - source: %s
    path: skills/alpha
    ref: "^1.0.0"
  - source: %s
    path: skills/beta
    ref: "^1.0.0"
`, first, second))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	updateLatest = true
	if err := updateCmd.RunE(updateCmd, []string{"alpha"}); err != nil {
		t.Fatalf("update --latest alpha: %v", err)
	}

	if got := installedSkillVersion(t, dir, "alpha"); got != "v2.0.0" {
		t.Errorf("alpha = %q, want v2.0.0", got)
	}
	if got := installedSkillVersion(t, dir, "beta"); got != "v1.0.0" {
		t.Errorf("beta = %q, want v1.0.0: it was not named", got)
	}
}

func TestUpdate_ReportsTheVersionMovement(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0")
	rangeManifest(t, source, "tool", "^1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	tagSourceVersion(t, strings.TrimPrefix(source, "git::"), "tool", "v1.1.0")

	logs := captureLog(t)
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	if !strings.Contains(logs.String(), "v1.0.0 -> v1.1.0") {
		t.Errorf("expected the version movement in the report, got:\n%s", logs.String())
	}
}

// A range that no tag satisfies has to fail loudly rather than fall back to
// some other commit.
func TestInstall_UnsatisfiableRangeFails(t *testing.T) {
	newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0")
	rangeManifest(t, source, "tool", "^9.0.0")

	err := installCmd.RunE(installCmd, nil)
	if err == nil {
		t.Fatal("expected an error for a range nothing satisfies")
	}
	if !strings.Contains(err.Error(), "^9.0.0") {
		t.Errorf("the error should name the range, got: %v", err)
	}
}

// A literal tag must stay put even as newer tags appear, which is the whole
// difference between naming a tag and naming a range.
func TestUpdate_LiteralTagDoesNotMove(t *testing.T) {
	dir := newProjectDir(t)
	source := newTaggedSource(t, "tool", "v1.0.0")
	rangeManifest(t, source, "tool", "v1.0.0")

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install: %v", err)
	}

	tagSourceVersion(t, strings.TrimPrefix(source, "git::"), "tool", "v2.0.0")

	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update: %v", err)
	}

	if got := installedSkillVersion(t, dir, "tool"); got != "v1.0.0" {
		t.Errorf("installed %q, want the pinned v1.0.0", got)
	}

	lock, err := lockfile.Read(LockfileName)
	if err != nil {
		t.Fatal(err)
	}
	if entry := lock.FindEntry("tool"); entry.ResolvedRef != "" {
		t.Errorf("ResolvedRef = %q, want empty for a literal tag", entry.ResolvedRef)
	}
}
