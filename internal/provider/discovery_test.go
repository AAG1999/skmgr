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

package provider

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// markSkill creates a directory holding the given marker file.
func markSkill(t *testing.T, root string, rel string, marker string) string {
	t.Helper()

	dir := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, marker), []byte("# "+rel+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// fetchLocal runs the local provider against a source tree.
func fetchLocal(t *testing.T, source string, path string) (FetchResult, error) {
	t.Helper()

	return (&LocalProvider{}).Fetch(
		types.SkillDependency{Name: "probe", Source: source, Path: path},
		t.TempDir(),
		FetchOptions{},
	)
}

// names returns the base name of every returned directory, sorted.
func names(dirs []string) []string {
	out := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		out = append(out, filepath.Base(dir))
	}
	slices.Sort(out)
	return out
}

// A pattern the user wrote is an instruction about where to look. Dropping a
// match because its parent folder has the "wrong" name reads as skmgr ignoring
// what was asked for.
func TestLocalFetch_WildcardMatchesAnyLayout(t *testing.T) {
	root := t.TempDir()
	markSkill(t, root, "packages/alpha/agent", "SKILL.md")
	markSkill(t, root, "packages/beta/agent", "SKILL.md")

	result, err := fetchLocal(t, root, "packages/*/agent")
	if err != nil {
		t.Fatalf("a wildcard should match regardless of layout: %v", err)
	}

	if got := len(result.SourceDirs); got != 2 {
		t.Fatalf("matched %d directories, want 2: %v", got, result.SourceDirs)
	}
}

// A rule found by a pattern is treated the same way as a skill.
func TestLocalFetch_WildcardFindsRulesOutsideRulesDir(t *testing.T) {
	root := t.TempDir()
	markSkill(t, root, "guidance/house-style", "AGENTS.md")

	result, err := fetchLocal(t, root, "guidance/*")
	if err != nil {
		t.Fatalf("a wildcard should match a rule anywhere: %v", err)
	}

	if got := names(result.SourceDirs); !slices.Equal(got, []string{"house-style"}) {
		t.Errorf("matched %v, want [house-style]", got)
	}
}

// Auto-discovery walks the whole tree with nothing pointing it anywhere, so the
// convention is the only thing keeping a quoted SKILL.md in a docs folder from
// being installed as a real skill.
func TestLocalFetch_AutoDiscoveryKeepsTheConvention(t *testing.T) {
	root := t.TempDir()
	markSkill(t, root, "skills/real", "SKILL.md")
	markSkill(t, root, "docs/examples/pretend", "SKILL.md")

	result, err := fetchLocal(t, root, "")
	if err != nil {
		t.Fatalf("auto-discovery should find the conventional skill: %v", err)
	}

	if got := names(result.SourceDirs); !slices.Equal(got, []string{"real"}) {
		t.Errorf("discovered %v, want only [real]", got)
	}
}

func TestLocalFetch_AutoDiscoveryFindsRules(t *testing.T) {
	root := t.TempDir()
	markSkill(t, root, "rules/style", "AGENTS.md")
	markSkill(t, root, "notes/style", "AGENTS.md")

	result, err := fetchLocal(t, root, "")
	if err != nil {
		t.Fatalf("auto-discovery should find the conventional rule: %v", err)
	}

	if got := names(result.SourceDirs); !slices.Equal(got, []string{"style"}) {
		t.Errorf("discovered %v, want only the one under rules/", got)
	}
	if len(result.SourceDirs) != 1 {
		t.Errorf("discovered %d directories, want 1: %v", len(result.SourceDirs), result.SourceDirs)
	}
}

// A pattern that matches nothing usable is an error, not an empty install.
func TestLocalFetch_WildcardWithoutMarkersFails(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "packages", "alpha"), 0755); err != nil {
		t.Fatal(err)
	}

	_, err := fetchLocal(t, root, "packages/*")
	if err == nil {
		t.Fatal("a pattern matching no skill should fail")
	}
}

func TestSkillCollector_SkipsTheGitDirectory(t *testing.T) {
	root := t.TempDir()
	markSkill(t, root, "skills/real", "SKILL.md")
	markSkill(t, root, ".git/skills/leftover", "SKILL.md")

	collector := &skillCollector{requireConventionalParent: true}
	if err := collector.walk(root); err != nil {
		t.Fatal(err)
	}

	if got := names(collector.dirs); !slices.Equal(got, []string{"real"}) {
		t.Errorf("collected %v, want only [real]", got)
	}
}

func TestSkillCollector_IgnoresDirectoriesWithoutAMarker(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "empty"), 0755); err != nil {
		t.Fatal(err)
	}

	collector := &skillCollector{}
	collector.consider(filepath.Join(root, "empty"))

	if len(collector.dirs) != 0 {
		t.Errorf("collected %v, want nothing", collector.dirs)
	}
}

func TestSkillCollector_ConsiderMatchesSkipsFiles(t *testing.T) {
	root := t.TempDir()
	dir := markSkill(t, root, "skills/one", "SKILL.md")
	file := filepath.Join(dir, "SKILL.md")

	collector := &skillCollector{}
	collector.considerMatches([]string{dir, file, filepath.Join(root, "missing")})

	if got := names(collector.dirs); !slices.Equal(got, []string{"one"}) {
		t.Errorf("collected %v, want only [one]", got)
	}
}
