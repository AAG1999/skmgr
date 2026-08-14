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

package linker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// stubSymlink replaces symlink creation for the duration of a test. Windows
// permission failures cannot be produced on the CI machines that run this
// suite, so the error has to be injected to cover the fallback chain.
func stubSymlink(t *testing.T, err error) {
	t.Helper()

	original := symlink
	symlink = func(target string, linkPath string) error { return err }
	t.Cleanup(func() { symlink = original })
}

// stubJunction replaces junction creation, which is a no-op off Windows.
func stubJunction(t *testing.T, fn func(target string, linkPath string) error) {
	t.Helper()

	original := junction
	junction = fn
	t.Cleanup(func() { junction = original })
}

// skillDir builds a small directory to stand in for an installed skill.
func skillDir(t *testing.T, root string, name string) string {
	t.Helper()

	dir := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "extra.md"), []byte("extra\n"), 0644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// On Windows a symlink needs Developer Mode or elevation, but a junction does
// not. Falling back to one keeps installs working on a stock machine.
func TestCreateSymlink_FallsBackToAJunction(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	stubSymlink(t, os.ErrPermission)

	var gotTarget, gotLink string
	stubJunction(t, func(target string, linkPath string) error {
		gotTarget, gotLink = target, linkPath
		return os.MkdirAll(linkPath, 0755)
	})

	if err := createSymlink(target, link); err != nil {
		t.Fatalf("createSymlink should have fallen back to a junction: %v", err)
	}

	if gotLink != link {
		t.Errorf("junction link = %q, want %q", gotLink, link)
	}
	// Junctions only accept absolute paths, unlike the symlinks we prefer.
	if !filepath.IsAbs(gotTarget) {
		t.Errorf("junction target = %q, want an absolute path", gotTarget)
	}
}

// When neither a symlink nor a junction is available, a copy is better than a
// failed install — the agent still sees the skill.
func TestCreateSymlink_CopiesWhenNoLinkIsPossible(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	stubSymlink(t, os.ErrPermission)
	stubJunction(t, func(string, string) error { return errors.New("no junctions here") })

	if err := createSymlink(target, link); err != nil {
		t.Fatalf("createSymlink should have copied: %v", err)
	}

	for _, rel := range []string{"SKILL.md", filepath.Join("nested", "extra.md")} {
		if _, err := os.Stat(filepath.Join(link, rel)); err != nil {
			t.Errorf("the copy is missing %s: %v", rel, err)
		}
	}

	if !isManagedFallback(link) {
		t.Error("the copy should be marked so a later run can tell it apart from a user's directory")
	}
}

// A copy goes stale as soon as the skill changes, so a later sync has to be
// able to replace it rather than refusing because it is not a symlink.
func TestCreateSymlink_RefreshesAPreviousCopy(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	stubSymlink(t, os.ErrPermission)
	stubJunction(t, func(string, string) error { return errors.New("no junctions here") })

	if err := createSymlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(target, "SKILL.md"), []byte("# updated\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := createSymlink(target, link); err != nil {
		t.Fatalf("a second sync should refresh the copy: %v", err)
	}

	contents, err := os.ReadFile(filepath.Join(link, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "# updated\n" {
		t.Errorf("the copy still holds the old content: %q", contents)
	}
}

// Falling back on a permission error is deliberate. Any other failure means
// something is genuinely wrong, and quietly copying would hide it.
func TestCreateSymlink_DoesNotFallBackOnOtherErrors(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	stubSymlink(t, errors.New("disk on fire"))
	stubJunction(t, func(string, string) error {
		t.Error("a junction should not be attempted for a non-permission failure")
		return nil
	})

	err := createSymlink(target, link)
	if err == nil {
		t.Fatal("expected the error to surface")
	}
	if _, statErr := os.Stat(link); statErr == nil {
		t.Error("nothing should have been created at the link path")
	}
}

// A directory the user made themselves is not ours to replace.
func TestCreateSymlink_LeavesAnUnmanagedDirectoryAlone(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	if err := os.MkdirAll(link, 0755); err != nil {
		t.Fatal(err)
	}
	precious := filepath.Join(link, "notes.md")
	if err := os.WriteFile(precious, []byte("mine\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := createSymlink(target, link); err == nil {
		t.Fatal("expected createSymlink to refuse a real directory")
	}

	if _, err := os.Stat(precious); err != nil {
		t.Errorf("the user's file should be untouched: %v", err)
	}
}

// A link recorded as an absolute path by an older skmgr should be rewritten,
// so a project converges on relative links without anyone intervening.
func TestCreateSymlink_RewritesAnAbsoluteLink(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	if err := os.MkdirAll(filepath.Dir(link), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	if err := createSymlink(target, link); err != nil {
		t.Fatal(err)
	}

	stored, err := os.Readlink(link)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.IsAbs(stored) {
		t.Errorf("link is still absolute: %q", stored)
	}
	if got, _ := linkTarget(link); got != target {
		t.Errorf("linkTarget = %q, want %q", got, target)
	}
}

// The whole point of relative links: the tree can be moved and the links still
// resolve. A container that mounts the project elsewhere is the common case.
func TestCreateSymlink_SurvivesMovingTheTree(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	if err := createSymlink(target, link); err != nil {
		t.Fatal(err)
	}

	moved := filepath.Join(t.TempDir(), "relocated")
	if err := os.Rename(root, moved); err != nil {
		t.Skipf("cannot move the tree on this filesystem: %v", err)
	}

	movedLink := filepath.Join(moved, "agent", "greeter")
	if !linkTargetExists(movedLink) {
		t.Error("the link should still resolve after the project moved")
	}
	if _, err := os.Stat(filepath.Join(movedLink, "SKILL.md")); err != nil {
		t.Errorf("the skill should be reachable through the moved link: %v", err)
	}
}

// Regression guard for a link being judged from the process working directory:
// a relative link looks broken from anywhere but its own directory, and skmgr
// deletes links it believes are broken.
func TestLinkTargetExists_IgnoresTheWorkingDirectory(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	if err := createSymlink(target, link); err != nil {
		t.Fatal(err)
	}

	elsewhere := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(elsewhere); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	if !linkTargetExists(link) {
		t.Error("an intact link must not look broken just because the working directory moved")
	}
}

func TestLinkTargetExists_ReportsABrokenLink(t *testing.T) {
	root := t.TempDir()
	target := skillDir(t, root, "greeter")
	link := filepath.Join(root, "agent", "greeter")

	if err := createSymlink(target, link); err != nil {
		t.Fatal(err)
	}
	if err := os.RemoveAll(target); err != nil {
		t.Fatal(err)
	}

	if linkTargetExists(link) {
		t.Error("a link whose target is gone should report as broken")
	}
}

func TestRelativeTarget(t *testing.T) {
	root := t.TempDir()

	cases := []struct {
		name   string
		target string
		link   string
		want   string
	}{
		{
			name:   "sibling",
			target: filepath.Join(root, "a.txt"),
			link:   filepath.Join(root, "b.txt"),
			want:   "a.txt",
		},
		{
			name:   "up one level",
			target: filepath.Join(root, ".agents", "skills", "greeter"),
			link:   filepath.Join(root, ".cursor", "skills", "greeter"),
			want:   filepath.Join("..", "..", ".agents", "skills", "greeter"),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := relativeTarget(tc.target, tc.link); got != tc.want {
				t.Errorf("relativeTarget = %q, want %q", got, tc.want)
			}
		})
	}
}
