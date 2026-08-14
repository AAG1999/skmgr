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
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func requireSymlinks(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs elevated privileges on Windows")
	}
}

// Skills legitimately ship symlinks (shared assets, helper binaries). They used
// to be dropped silently, which installed an incomplete tree.
func TestInstall_PreservesInternalSymlinks(t *testing.T) {
	requireSymlinks(t)

	source := t.TempDir()
	skillDir := filepath.Join(source, "skills", "with-link")
	if err := os.MkdirAll(filepath.Join(skillDir, "assets"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "assets", "real.txt"), []byte("payload"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join("assets", "real.txt"), filepath.Join(skillDir, "link.txt")); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	installer := NewInstaller(root)

	_, err := installer.Install(ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name:  "with-link",
			Type:  types.TypeSkill,
			Scope: types.ScopeProject,
		},
		SourceDir: skillDir,
	})
	if err != nil {
		t.Fatalf("Install() returned error: %v", err)
	}

	installedLink := filepath.Join(root, ".agents", "skills", "with-link", "link.txt")
	info, err := os.Lstat(installedLink)
	if err != nil {
		t.Fatalf("symlink was not installed: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Error("entry was installed as a regular file instead of a symlink")
	}

	contents, err := os.ReadFile(installedLink)
	if err != nil {
		t.Fatalf("installed symlink does not resolve: %v", err)
	}
	if string(contents) != "payload" {
		t.Errorf("resolved content = %q, want %q", contents, "payload")
	}
}

// A symlink escaping the skill would smuggle host files into .agents, so the
// install must fail loudly rather than copy it.
func TestInstall_RejectsSymlinkEscapingSkill(t *testing.T) {
	requireSymlinks(t)

	root := t.TempDir()
	outside := filepath.Join(root, "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0644); err != nil {
		t.Fatal(err)
	}

	skillDir := filepath.Join(root, "source", "skills", "escaper")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("skill"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(skillDir, "leak.txt")); err != nil {
		t.Fatal(err)
	}

	project := t.TempDir()
	installer := NewInstaller(project)

	_, err := installer.Install(ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name:  "escaper",
			Type:  types.TypeSkill,
			Scope: types.ScopeProject,
		},
		SourceDir: skillDir,
	})

	if err == nil {
		t.Fatal("Install() accepted a symlink pointing outside the skill, want error")
	}
	if !strings.Contains(err.Error(), "points outside") {
		t.Errorf("error = %q, want it to mention the escaping symlink", err.Error())
	}
}

// Install joins the name onto a base directory, so unsafe names are rejected
// even if they somehow bypassed manifest validation.
func TestInstall_RejectsUnsafeName(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("skill"), 0644); err != nil {
		t.Fatal(err)
	}

	installer := NewInstaller(t.TempDir())

	_, err := installer.Install(ResolvedSkill{
		SkillDependency: types.SkillDependency{
			Name:  "../escape",
			Type:  types.TypeSkill,
			Scope: types.ScopeProject,
		},
		SourceDir: source,
	})

	if err == nil {
		t.Fatal("Install() accepted an unsafe name, want error")
	}
}
