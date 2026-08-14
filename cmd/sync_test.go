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
	"testing"
)

func TestSync_NoManifest(t *testing.T) {
	newProjectDir(t)

	if err := syncCmd.RunE(syncCmd, nil); err == nil {
		t.Fatal("sync without a manifest should fail")
	}
}

func TestSync_InstallsDeclaredSkills(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "my-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/my-skill
`, source))

	if err := syncCmd.RunE(syncCmd, nil); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "my-skill", "SKILL.md")); err != nil {
		t.Errorf("sync did not install the skill: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(dir, ".cursor", "skills", "my-skill")); err != nil {
		t.Errorf("sync did not create the agent link: %v", err)
	}
}

// Sync exists to repair disk state, so it must restore a link deleted by hand.
func TestSync_RestoresDeletedAgentLink(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "my-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/my-skill
`, source))

	if err := syncCmd.RunE(syncCmd, nil); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	link := filepath.Join(dir, ".cursor", "skills", "my-skill")
	if err := os.Remove(link); err != nil {
		t.Fatalf("removing link for the test: %v", err)
	}

	if err := syncCmd.RunE(syncCmd, nil); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	if _, err := os.Lstat(link); err != nil {
		t.Errorf("sync did not restore the deleted agent link: %v", err)
	}
}

func TestSync_RemovesOrphanedCanonicalDir(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "declared")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/declared
`, source))

	if err := syncCmd.RunE(syncCmd, nil); err != nil {
		t.Fatalf("sync failed: %v", err)
	}

	// Something the manifest never mentioned.
	stray := filepath.Join(dir, ".agents", "skills", "stray")
	if err := os.MkdirAll(stray, 0755); err != nil {
		t.Fatal(err)
	}

	if err := syncCmd.RunE(syncCmd, nil); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	if _, err := os.Stat(stray); !os.IsNotExist(err) {
		t.Error("sync should have removed the undeclared canonical directory")
	}
}
