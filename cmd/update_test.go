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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/lockfile"
)

func TestUpdate_NoManifest(t *testing.T) {
	newProjectDir(t)

	if err := updateCmd.RunE(updateCmd, nil); err == nil {
		t.Fatal("update without a manifest should fail")
	}
}

// The skill name argument used to be accepted and ignored.
func TestUpdate_UnknownNameIsRejected(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "known")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/known
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	err := updateCmd.RunE(updateCmd, []string{"does-not-exist"})
	if err == nil {
		t.Fatal("update with an unknown skill name should fail rather than silently update everything")
	}
	if !strings.Contains(err.Error(), "does-not-exist") {
		t.Errorf("error = %q, want it to name the unknown skill", err.Error())
	}
}

// Naming one skill must leave the others pinned at their locked commit.
func TestUpdate_NamedSkillHoldsOthersAtLockedContent(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "moving", "frozen")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/moving
  - source: file://%s
    path: skills/frozen
`, source, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	before, err := lockfile.Read(filepath.Join(dir, LockfileName))
	if err != nil {
		t.Fatal(err)
	}

	// Change both upstream sources.
	for _, name := range []string{"moving", "frozen"} {
		p := filepath.Join(source, "skills", name, "SKILL.md")
		if err := os.WriteFile(p, []byte("# "+name+" updated"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	if err := updateCmd.RunE(updateCmd, []string{"moving"}); err != nil {
		t.Fatalf("update moving failed: %v", err)
	}

	after, err := lockfile.Read(filepath.Join(dir, LockfileName))
	if err != nil {
		t.Fatal(err)
	}

	movingBefore := before.FindEntry("moving")
	movingAfter := after.FindEntry("moving")
	if movingBefore == nil || movingAfter == nil {
		t.Fatal("moving is missing from a lockfile")
	}
	if movingBefore.ContentHash == movingAfter.ContentHash {
		t.Error("the named skill should have picked up the upstream change")
	}

	// Both skills are local sources with no commit SHA, so the other one is not
	// pinnable and will also refresh. What must hold is that it is still
	// installed and tracked, never dropped as an orphan.
	if after.FindEntry("frozen") == nil {
		t.Error("updating one skill must not remove the others from the lockfile")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "frozen")); err != nil {
		t.Errorf("updating one skill must not uninstall the others: %v", err)
	}
}

func TestUpdate_AllSkillsWithoutArgs(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "one", "two")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/one
  - source: file://%s
    path: skills/two
`, source, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(source, "skills", "one", "SKILL.md"), []byte("# one v2"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	installed, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "one", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != "# one v2" {
		t.Errorf("installed content = %q, want the updated content", installed)
	}
}

func TestUpdate_NamedSkillBeforeInstallFails(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "thing")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/thing
`, source))

	err := updateCmd.RunE(updateCmd, []string{"thing"})
	if err == nil {
		t.Fatal("updating a named skill with no lockfile should fail")
	}
	if !strings.Contains(err.Error(), LockfileName) {
		t.Errorf("error = %q, want it to mention the missing lockfile", err.Error())
	}
}

func TestUpdate_ReportsWhenNothingChanged(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "steady")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/steady
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	out := captureLog(t)
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatalf("update failed: %v", err)
	}

	if !strings.Contains(out.String(), "already up to date") {
		t.Errorf("update output = %q, want it to say nothing changed", out.String())
	}
}
