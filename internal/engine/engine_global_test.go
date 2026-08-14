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

// isolateHome points the user's home directory at a temp dir so global-scope
// operations in tests never touch the real ~/.agents or ~/.skmgr.
func isolateHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return home
}

// newTestEngine builds an engine whose skmgr state lives under the isolated
// home rather than the developer's real one.
func newTestEngine(t *testing.T, projectRoot, cacheDir string) *Engine {
	t.Helper()
	e := NewEngine(projectRoot, cacheDir)
	e.SkmgrHome = types.SkmgrHome()
	return e
}

// writeSkillSource creates a minimal on-disk skill the local provider accepts.
func writeSkillSource(t *testing.T, root, name string) string {
	t.Helper()
	dir := filepath.Join(root, "skills", name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating skill source: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}
	return dir
}

// Regression for the bug where any project install with no global skills wiped
// every globally installed skill in the shared home tree.
func TestSync_DoesNotWipeGlobalSkillsOfOtherProjects(t *testing.T) {
	home := isolateHome(t)

	// Another project already owns a global skill.
	otherProject := t.TempDir()
	globalSkillDir := filepath.Join(home, ".agents", "skills", "owned-by-other")
	if err := os.MkdirAll(globalSkillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalSkillDir, "SKILL.md"), []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	ledgerPath := lockfile.GlobalLedgerPath(types.SkmgrHome())
	ledger, err := lockfile.ReadGlobalLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	ledger.SetProject(otherProject, []string{"owned-by-other"})
	if err := lockfile.WriteGlobalLedger(ledgerPath, ledger); err != nil {
		t.Fatal(err)
	}

	// Now a completely unrelated project installs a project-scoped skill.
	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "local-skill")

	e := newTestEngine(t, root, t.TempDir())
	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{
				Name:   "local-skill",
				Source: "file://" + source,
				Path:   "skills/local-skill",
				Scope:  types.ScopeProject,
			},
		},
	}

	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(globalSkillDir, "SKILL.md")); os.IsNotExist(err) {
		t.Fatal("a project install with no global skills deleted another project's global skill")
	}
}

// On the first run after upgrading there is no ledger, so pre-existing global
// installs must be adopted rather than treated as orphans.
func TestSync_AdoptsPreExistingGlobalSkills(t *testing.T) {
	home := isolateHome(t)

	legacy := filepath.Join(home, ".agents", "skills", "installed-before-ledger")
	if err := os.MkdirAll(legacy, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacy, "SKILL.md"), []byte("legacy"), 0644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "local-skill")

	e := newTestEngine(t, root, t.TempDir())
	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{
				Name:   "local-skill",
				Source: "file://" + source,
				Path:   "skills/local-skill",
				Scope:  types.ScopeProject,
			},
		},
	}

	// Two syncs: the first creates the ledger, the second prunes against it.
	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("first Sync failed: %v", err)
	}
	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(legacy, "SKILL.md")); os.IsNotExist(err) {
		t.Error("a global skill that predates the ledger was deleted instead of adopted")
	}

	ledger, err := lockfile.ReadGlobalLedger(lockfile.GlobalLedgerPath(types.SkmgrHome()))
	if err != nil {
		t.Fatal(err)
	}
	if !ledger.AllNames()["installed-before-ledger"] {
		t.Error("pre-existing global skill was not recorded in the ledger")
	}
}

// Adoption used to happen only when the ledger file was absent, so a global
// skill that appeared after some other project had created the ledger was
// treated as an orphan and deleted.
func TestSync_AdoptsGlobalSkillsThatAppearAfterTheLedgerExists(t *testing.T) {
	home := isolateHome(t)

	// Another project installs first, which is what creates the ledger.
	ledgerPath := lockfile.GlobalLedgerPath(types.SkmgrHome())
	ledger, err := lockfile.ReadGlobalLedger(ledgerPath)
	if err != nil {
		t.Fatal(err)
	}
	ledger.SetProject(t.TempDir(), []string{"owned-by-other"})
	if err := lockfile.WriteGlobalLedger(ledgerPath, ledger); err != nil {
		t.Fatal(err)
	}

	// Only now does an unrelated global skill turn up in the shared tree.
	unmanaged := filepath.Join(home, ".agents", "skills", "placed-by-hand")
	if err := os.MkdirAll(unmanaged, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(unmanaged, "SKILL.md"), []byte("keep me"), 0644); err != nil {
		t.Fatal(err)
	}

	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "local-skill")

	e := newTestEngine(t, root, t.TempDir())
	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{
				Name:   "local-skill",
				Source: "file://" + source,
				Path:   "skills/local-skill",
				Scope:  types.ScopeProject,
			},
		},
	}

	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("first Sync failed: %v", err)
	}
	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(unmanaged, "SKILL.md")); os.IsNotExist(err) {
		t.Error("a global skill skmgr never installed was deleted instead of adopted")
	}
}

// Adopting unknown installs must not make a project's own global skills
// permanent: dropping one from the manifest still has to prune it.
func TestSync_DroppedGlobalSkillIsStillPruned(t *testing.T) {
	home := isolateHome(t)

	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "global-skill")

	e := newTestEngine(t, root, t.TempDir())
	withSkill := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{
				Name:   "global-skill",
				Source: "file://" + source,
				Path:   "skills/global-skill",
				Scope:  types.ScopeGlobal,
			},
		},
	}

	lock, err := e.Sync(withSkill, nil, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	canonical := filepath.Join(home, ".agents", "skills", "global-skill")
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("global skill was not installed: %v", err)
	}

	// The same project now declares nothing at all.
	empty := &types.Manifest{Targets: []string{"cursor"}}
	if _, err := e.Sync(empty, lock, SyncOptions{}); err != nil {
		t.Fatalf("second Sync failed: %v", err)
	}

	if _, err := os.Stat(canonical); !os.IsNotExist(err) {
		t.Error("a global skill dropped from the manifest was adopted instead of pruned")
	}
}

// A global-scope skill must land in the home tree, never in the project.
func TestSync_GlobalScopeInstallsIntoHome(t *testing.T) {
	home := isolateHome(t)

	root := t.TempDir()
	source := t.TempDir()
	writeSkillSource(t, source, "global-skill")

	e := newTestEngine(t, root, t.TempDir())
	manifest := &types.Manifest{
		Targets: []string{"cursor"},
		Skills: []types.SkillDependency{
			{
				Name:   "global-skill",
				Source: "file://" + source,
				Path:   "skills/global-skill",
				Scope:  types.ScopeGlobal,
			},
		},
	}

	if _, err := e.Sync(manifest, nil, SyncOptions{}); err != nil {
		t.Fatalf("Sync failed: %v", err)
	}

	canonical := filepath.Join(home, ".agents", "skills", "global-skill", "SKILL.md")
	if _, err := os.Stat(canonical); os.IsNotExist(err) {
		t.Errorf("global skill was not installed into the home canonical dir (%s)", canonical)
	}

	homeLink := filepath.Join(home, ".cursor", "skills", "global-skill")
	if _, err := os.Lstat(homeLink); os.IsNotExist(err) {
		t.Errorf("global skill was not linked into the home agent dir (%s)", homeLink)
	}

	projectLink := filepath.Join(root, ".cursor", "skills", "global-skill")
	if _, err := os.Lstat(projectLink); err == nil {
		t.Error("global skill leaked a symlink into the project agent dir")
	}
}

func TestGlobalLedger_UnionAcrossProjects(t *testing.T) {
	dir := t.TempDir()
	path := lockfile.GlobalLedgerPath(dir)

	ledger, err := lockfile.ReadGlobalLedger(path)
	if err != nil {
		t.Fatal(err)
	}

	ledger.SetProject("/projects/a", []string{"skill-a", "shared"})
	ledger.SetProject("/projects/b", []string{"skill-b", "shared"})

	if err := lockfile.WriteGlobalLedger(path, ledger); err != nil {
		t.Fatal(err)
	}

	reloaded, err := lockfile.ReadGlobalLedger(path)
	if err != nil {
		t.Fatal(err)
	}

	names := reloaded.AllNames()
	for _, want := range []string{"skill-a", "skill-b", "shared"} {
		if !names[want] {
			t.Errorf("AllNames() missing %q", want)
		}
	}

	// Dropping a project removes only the names it uniquely owned.
	reloaded.SetProject("/projects/a", nil)
	names = reloaded.AllNames()
	if names["skill-a"] {
		t.Error("skill-a should be gone after its owning project was cleared")
	}
	if !names["shared"] {
		t.Error("shared is still owned by project b and must survive")
	}
}
