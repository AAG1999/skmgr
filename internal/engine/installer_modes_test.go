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
	"runtime"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// installSkillTree installs a skill built by the given function and returns the
// installed directory.
func installSkillTree(t *testing.T, name string, build func(dir string)) string {
	t.Helper()

	source := t.TempDir()
	skillDir := filepath.Join(source, "skills", name)
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("# skill\n"), 0644); err != nil {
		t.Fatal(err)
	}

	build(skillDir)

	root := t.TempDir()
	if _, err := NewInstaller(root).Install(ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name:  name,
			Type:  types.TypeSkill,
			Scope: types.ScopeProject,
		},
		SourceDir: skillDir,
	}); err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}

	return filepath.Join(root, ".agents", "skills", name)
}

// Plenty of skills ship a helper script and a SKILL.md that says to run it.
// Installing the script without its executable bit produces a skill that looks
// complete and fails the moment the agent uses it.
func TestInstall_PreservesTheExecutableBit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no executable bit to preserve")
	}

	installed := installSkillTree(t, "with-script", func(dir string) {
		script := filepath.Join(dir, "run.sh")
		if err := os.WriteFile(script, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
			t.Fatal(err)
		}
		plain := filepath.Join(dir, "notes.md")
		if err := os.WriteFile(plain, []byte("notes\n"), 0644); err != nil {
			t.Fatal(err)
		}
	})

	script, err := os.Stat(filepath.Join(installed, "run.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if script.Mode().Perm()&0100 == 0 {
		t.Errorf("run.sh installed as %v, the owner executable bit was lost", script.Mode().Perm())
	}

	// The bit is copied, not added: a plain file must not come out executable.
	plain, err := os.Stat(filepath.Join(installed, "notes.md"))
	if err != nil {
		t.Fatal(err)
	}
	if plain.Mode().Perm()&0111 != 0 {
		t.Errorf("notes.md installed as %v, it should not be executable", plain.Mode().Perm())
	}
}

func TestInstall_PreservesTheExecutableBitInSubdirectories(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows has no executable bit to preserve")
	}

	installed := installSkillTree(t, "nested-script", func(dir string) {
		scripts := filepath.Join(dir, "scripts")
		if err := os.MkdirAll(scripts, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(scripts, "tool.py"), []byte("#!/usr/bin/env python3\n"), 0755); err != nil {
			t.Fatal(err)
		}
	})

	info, err := os.Stat(filepath.Join(installed, "scripts", "tool.py"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0100 == 0 {
		t.Errorf("scripts/tool.py installed as %v, the executable bit was lost", info.Mode().Perm())
	}
}

// Installed content has to be readable by the agent, which in a container or a
// shared CI image may not be the user that ran skmgr.
func TestInstall_LeavesContentReadable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission bits do not carry the same meaning on Windows")
	}

	installed := installSkillTree(t, "readable", func(dir string) {
		if err := os.WriteFile(filepath.Join(dir, "reference.md"), []byte("ref\n"), 0644); err != nil {
			t.Fatal(err)
		}
	})

	for _, name := range []string{"SKILL.md", "reference.md"} {
		info, err := os.Stat(filepath.Join(installed, name))
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm()&0044 == 0 {
			t.Errorf("%s installed as %v, which the agent may not be able to read", name, info.Mode().Perm())
		}
	}

	dir, err := os.Stat(installed)
	if err != nil {
		t.Fatal(err)
	}
	if dir.Mode().Perm()&0055 == 0 {
		t.Errorf("the skill directory is %v, which the agent may not be able to enter", dir.Mode().Perm())
	}
}

// Ownership is deliberately not preserved: skmgr copies as the user running it
// and does not chown, because doing so would need privileges a package manager
// has no business holding. This test records that, so a future change to copy
// ownership is a deliberate decision rather than an accident.
func TestInstall_FilesAreOwnedByTheInstallingUser(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX ownership does not apply")
	}

	installed := installSkillTree(t, "owned", func(dir string) {})

	info, err := os.Stat(filepath.Join(installed, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}

	owner, ok := fileOwner(info)
	if !ok {
		t.Skip("ownership is not reported on this platform")
	}
	if owner != os.Getuid() {
		t.Errorf("installed file is owned by uid %d, want the installing user %d", owner, os.Getuid())
	}
}
