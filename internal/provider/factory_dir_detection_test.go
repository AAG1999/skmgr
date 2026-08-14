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
	"runtime"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// chdir moves into a directory for the duration of a test, which is the only
// way to exercise the relative-path branch of source detection.
func chdir(t *testing.T, dir string) {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
}

// A bare relative path is the shape people write when vendoring skills inside
// their own repository. Reading it as a git URL yields a clone failure for a
// directory that is sitting right there.
func TestGetProvider_BareRelativeDirectoryIsLocal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "vendor", "skills"), 0755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)

	if _, ok := GetProvider("vendor/skills").(*LocalProvider); !ok {
		t.Error("a bare path naming a real directory should use the local provider")
	}
}

// Nothing on disk means the source can only be a remote.
func TestGetProvider_BarePathThatIsNotADirectoryIsGit(t *testing.T) {
	chdir(t, t.TempDir())

	if _, ok := GetProvider("org/repo").(*GitProvider); !ok {
		t.Error("a bare path with nothing on disk should use the git provider")
	}
}

// A directory that happens to share a name with a remote must not shadow it,
// or a stray folder could silently redirect someone's dependency.
func TestGetProvider_RemoteFormsIgnoreTheDisk(t *testing.T) {
	root := t.TempDir()
	for _, decoy := range []string{
		filepath.Join("https:", "github.com", "org", "repo"),
		filepath.Join("org", "repo.git"),
	} {
		// A colon is reserved on Windows, so the decoy that mirrors a URL
		// cannot exist there at all. The rest still make the point, and every
		// assertion below is unchanged.
		if runtime.GOOS == "windows" && strings.Contains(decoy, ":") {
			continue
		}
		if err := os.MkdirAll(filepath.Join(root, decoy), 0755); err != nil {
			t.Fatal(err)
		}
	}
	chdir(t, root)

	remotes := []string{
		"https://github.com/org/repo",
		"git@github.com:org/repo.git",
		"ssh://git@example.com/org/repo",
		"org/repo.git",
	}

	for _, source := range remotes {
		if _, ok := GetProvider(source).(*GitProvider); !ok {
			t.Errorf("GetProvider(%q) should stay on the git provider", source)
		}
	}
}

// The explicit marker is how someone says "this local clone is a git
// repository, check out the ref", so it has to win over the disk check.
func TestGetProvider_GitPrefixBeatsDirectoryDetection(t *testing.T) {
	root := t.TempDir()
	mirror := filepath.Join(root, "mirror")
	if err := os.MkdirAll(mirror, 0755); err != nil {
		t.Fatal(err)
	}
	chdir(t, root)

	if _, ok := GetProvider(GitPrefix + "mirror").(*GitProvider); !ok {
		t.Error("git:: should force the git provider even for a real directory")
	}
}

// A repository with no skills/ or rules/ tree and no marker at the root is not
// a skill source. Installing the whole tree would copy a README and a build
// system into the agent's directory and look like it had worked.
func TestLocalFetch_RefusesToInstallAWholeTree(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "README.md"), []byte("# not a skill\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(source, "src"), 0755); err != nil {
		t.Fatal(err)
	}

	provider := &LocalProvider{}
	_, err := provider.Fetch(types.SkillDependency{Name: "whatever", Source: source}, t.TempDir(), FetchOptions{})
	if err == nil {
		t.Fatal("expected auto-discovery to refuse a source with no skills in it")
	}

	// The message has to say what to do next, since the fix is a manifest edit.
	for _, want := range []string{"SKILL.md", "path:"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should mention %q, got: %v", want, err)
		}
	}
}

// A single-skill repository keeps working: its root is the skill.
func TestLocalFetch_RootWithASkillMarkerIsInstalled(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "SKILL.md"), []byte("# solo\n"), 0644); err != nil {
		t.Fatal(err)
	}

	provider := &LocalProvider{}
	result, err := provider.Fetch(types.SkillDependency{Name: "solo", Source: source}, t.TempDir(), FetchOptions{})
	if err != nil {
		t.Fatalf("a single-skill source should install: %v", err)
	}

	if len(result.SourceDirs) != 1 || result.SourceDirs[0] != source {
		t.Errorf("SourceDirs = %v, want just the root", result.SourceDirs)
	}
}

func TestLocalFetch_RootWithARuleMarkerIsInstalled(t *testing.T) {
	source := t.TempDir()
	if err := os.WriteFile(filepath.Join(source, "AGENTS.md"), []byte("# guidance\n"), 0644); err != nil {
		t.Fatal(err)
	}

	provider := &LocalProvider{}
	result, err := provider.Fetch(types.SkillDependency{Name: "guidance", Source: source}, t.TempDir(), FetchOptions{})
	if err != nil {
		t.Fatalf("a single-rule source should install: %v", err)
	}
	if len(result.SourceDirs) != 1 {
		t.Errorf("SourceDirs = %v, want just the root", result.SourceDirs)
	}
}
