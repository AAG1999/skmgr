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
	"testing"

	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/types"
)

func stagedSkill(t *testing.T, files map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	for name, body := range files {
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func installFixture(name string, sourceDir string) ResolvedSkill {
	return ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name:   name,
			Source: "file://" + sourceDir,
			Type:   types.TypeSkill,
			Scope:  types.ScopeProject,
		},
		SourceDir: sourceDir,
	}
}

func TestInstall_ReplacesPreviousContent(t *testing.T) {
	project := t.TempDir()
	inst := NewInstaller(project)

	first := stagedSkill(t, map[string]string{
		"SKILL.md": "v1",
		"old.txt":  "gone in v2",
	})
	if _, err := inst.Install(installFixture("thing", first)); err != nil {
		t.Fatalf("first install: %v", err)
	}

	second := stagedSkill(t, map[string]string{"SKILL.md": "v2"})
	if _, err := inst.Install(installFixture("thing", second)); err != nil {
		t.Fatalf("second install: %v", err)
	}

	dest := filepath.Join(project, ".agents", "skills", "thing")

	body, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "v2" {
		t.Errorf("contents = %q, want the new version", body)
	}
	if _, err := os.Stat(filepath.Join(dest, "old.txt")); !os.IsNotExist(err) {
		t.Error("a file dropped between versions should not survive the reinstall")
	}
}

// A copy that fails part way used to leave a partial skill sitting in the
// canonical directory, indistinguishable from a good install.
func TestInstall_FailureLeavesPreviousInstallIntact(t *testing.T) {
	project := t.TempDir()
	inst := NewInstaller(project)

	good := stagedSkill(t, map[string]string{"SKILL.md": "the good version"})
	if _, err := inst.Install(installFixture("thing", good)); err != nil {
		t.Fatalf("first install: %v", err)
	}

	// A symlink escaping the skill is refused mid-copy, which is a realistic
	// way for the copy to fail after it has already written some files.
	bad := stagedSkill(t, map[string]string{"SKILL.md": "the bad version"})
	if err := os.Symlink(filepath.Join(project, "outside.txt"), filepath.Join(bad, "escape")); err != nil {
		t.Fatal(err)
	}

	if _, err := inst.Install(installFixture("thing", bad)); err == nil {
		t.Fatal("expected the install to fail on an escaping symlink")
	}

	dest := filepath.Join(project, ".agents", "skills", "thing")
	body, err := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	if err != nil {
		t.Fatalf("the previous install should still be there: %v", err)
	}
	if string(body) != "the good version" {
		t.Errorf("contents = %q, want the previous install untouched", body)
	}
	if _, err := os.Stat(filepath.Join(dest, "escape")); err == nil {
		t.Error("the failed install leaked a file into the canonical directory")
	}
}

// Staging happens inside the canonical tree so the final rename stays on one
// filesystem; it must not be left behind for the user to wonder about.
func TestInstall_LeavesNoStagingDirectory(t *testing.T) {
	project := t.TempDir()
	inst := NewInstaller(project)

	source := stagedSkill(t, map[string]string{"SKILL.md": "x"})
	if _, err := inst.Install(installFixture("thing", source)); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(filepath.Join(project, ".agents", ".staging")); !os.IsNotExist(err) {
		t.Error("the staging directory should be gone after a successful install")
	}

	entries, err := os.ReadDir(filepath.Join(project, ".agents", "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "thing" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("expected only the installed skill, got %v", names)
	}
}

// The hash must describe what actually landed on disk.
func TestInstall_HashMatchesInstalledContent(t *testing.T) {
	project := t.TempDir()
	inst := NewInstaller(project)

	source := stagedSkill(t, map[string]string{
		"SKILL.md":       "body",
		"docs/extra.md":  "more",
		"scripts/run.sh": "#!/bin/sh\n",
	})

	hash, err := inst.Install(installFixture("thing", source))
	if err != nil {
		t.Fatal(err)
	}

	dest := filepath.Join(project, ".agents", "skills", "thing")
	recomputed, err := lockfile.HashDirectory(dest)
	if err != nil {
		t.Fatal(err)
	}

	if hash != recomputed {
		t.Errorf("reported hash %q does not match the installed tree %q", hash, recomputed)
	}
}
