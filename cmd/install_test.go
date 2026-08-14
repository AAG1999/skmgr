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

func TestInstall_NoManifest(t *testing.T) {
	newProjectDir(t)

	err := installCmd.RunE(installCmd, nil)
	if err == nil {
		t.Fatal("install without a manifest should fail")
	}
	if !strings.Contains(err.Error(), "no manifest found") {
		t.Errorf("error = %q, want it to say no manifest was found", err.Error())
	}
}

func TestInstall_InstallsAndWritesLock(t *testing.T) {
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

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "my-skill", "SKILL.md")); err != nil {
		t.Errorf("skill was not installed into the canonical directory: %v", err)
	}

	lock, err := lockfile.Read(filepath.Join(dir, LockfileName))
	if err != nil {
		t.Fatalf("reading lockfile: %v", err)
	}
	if lock == nil || len(lock.Entries) != 1 {
		t.Fatalf("expected exactly one lock entry, got %+v", lock)
	}

	entry := lock.Entries[0]
	if entry.Name != "my-skill" {
		t.Errorf("lock entry name = %q, want %q", entry.Name, "my-skill")
	}
	// Source, Path and ResolvedAt used to be declared but never written.
	if entry.Source == "" {
		t.Error("lock entry is missing Source")
	}
	if entry.Path != "skills/my-skill" {
		t.Errorf("lock entry Path = %q, want %q", entry.Path, "skills/my-skill")
	}
	if entry.ResolvedAt == "" {
		t.Error("lock entry is missing ResolvedAt")
	}
	if entry.ContentHash == "" {
		t.Error("lock entry is missing ContentHash")
	}
}

// A manifest named skmgr.yaml must work exactly like skmgr.yml.
func TestInstall_AcceptsYamlExtension(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "yaml-skill")

	writeManifest(t, "skmgr.yaml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/yaml-skill
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install with skmgr.yaml failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "yaml-skill")); err != nil {
		t.Errorf("skill declared in skmgr.yaml was not installed: %v", err)
	}
}

func TestInstall_FrozenWithoutLockFails(t *testing.T) {
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

	installFrozen = true

	err := installCmd.RunE(installCmd, nil)
	if err == nil {
		t.Fatal("--frozen without a lockfile should fail")
	}
	if !strings.Contains(err.Error(), "frozen") {
		t.Errorf("error = %q, want it to mention frozen mode", err.Error())
	}
}

func TestInstall_FrozenSucceedsWithLock(t *testing.T) {
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

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("initial install failed: %v", err)
	}

	installFrozen = true
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("frozen install failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "my-skill")); err != nil {
		t.Errorf("frozen install did not keep the skill in place: %v", err)
	}
}

// A lockfile that exists but cannot be parsed is corruption. Treating it as
// absent would silently discard the pinned state on the next write.
func TestInstall_CorruptLockfileIsReported(t *testing.T) {
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

	writeManifest(t, LockfileName, "this: is: not: valid: yaml:\n  - [\n")

	err := installCmd.RunE(installCmd, nil)
	if err == nil {
		t.Fatal("a corrupt lockfile should stop the install, not be silently replaced")
	}
	if !strings.Contains(err.Error(), LockfileName) {
		t.Errorf("error = %q, want it to name the lockfile", err.Error())
	}
}

func TestInstall_RemovesOrphanWhenSkillDropped(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "keep-me", "drop-me")

	both := fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/keep-me
  - source: file://%s
    path: skills/drop-me
`, source, source)

	writeManifest(t, "skmgr.yml", both)
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("initial install failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "drop-me")); err != nil {
		t.Fatalf("drop-me should have been installed first: %v", err)
	}

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/keep-me
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("second install failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "drop-me")); !os.IsNotExist(err) {
		t.Error("drop-me should have been removed as an orphan")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "keep-me")); err != nil {
		t.Errorf("keep-me should still be installed: %v", err)
	}
}
