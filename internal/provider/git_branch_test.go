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
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func requireGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git is not installed")
	}
}

// runGit runs a git command in dir and fails the test if it errors.
func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test",
		"GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_CONFIG_SYSTEM=/dev/null",
	)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

// newUpstreamRepo builds a repo containing one skill and returns its path.
func newUpstreamRepo(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	runGit(t, dir, "init", "--quiet", "--initial-branch=main")

	skillDir := filepath.Join(dir, "skills", "demo")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("version one"), 0644); err != nil {
		t.Fatal(err)
	}

	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "--quiet", "-m", "first")

	return dir
}

// The cache used to check out the local branch created at clone time, which
// git fetch never fast-forwards, so a moving branch ref served a stale commit
// on every subsequent install.
func TestGitProvider_BranchRefPicksUpNewCommits(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()

	skill := types.SkillDependency{
		Name:   "demo",
		Source: upstream,
		Path:   "skills/demo",
		Ref:    "main",
	}

	p := &GitProvider{}

	first, err := p.Fetch(skill, cache, FetchOptions{})
	if err != nil {
		t.Fatalf("first Fetch failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(first.SourceDirs[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "version one" {
		t.Fatalf("first fetch content = %q, want %q", content, "version one")
	}

	// Move the branch forward upstream.
	if err := os.WriteFile(filepath.Join(upstream, "skills", "demo", "SKILL.md"), []byte("version two"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "--quiet", "-m", "second")
	wantSHA := runGit(t, upstream, "rev-parse", "HEAD")

	second, err := p.Fetch(skill, cache, FetchOptions{})
	if err != nil {
		t.Fatalf("second Fetch failed: %v", err)
	}

	content, err = os.ReadFile(filepath.Join(second.SourceDirs[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "version two" {
		t.Errorf("second fetch served stale content %q; the branch ref did not fast-forward", content)
	}
	if second.CommitSHA != wantSHA {
		t.Errorf("CommitSHA = %q, want the new upstream commit %q", second.CommitSHA, wantSHA)
	}
}

// A tag is immutable and must resolve to its own commit, not the branch tip.
func TestGitProvider_TagRefStaysPinned(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	runGit(t, upstream, "tag", "v1.0.0")
	taggedSHA := runGit(t, upstream, "rev-parse", "HEAD")

	// Advance the branch past the tag.
	if err := os.WriteFile(filepath.Join(upstream, "skills", "demo", "SKILL.md"), []byte("after tag"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "--quiet", "-m", "after tag")

	cache := t.TempDir()
	p := &GitProvider{}

	res, err := p.Fetch(types.SkillDependency{
		Name:   "demo",
		Source: upstream,
		Path:   "skills/demo",
		Ref:    "v1.0.0",
	}, cache, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if res.CommitSHA != taggedSHA {
		t.Errorf("CommitSHA = %q, want the tagged commit %q", res.CommitSHA, taggedSHA)
	}

	content, err := os.ReadFile(filepath.Join(res.SourceDirs[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "version one" {
		t.Errorf("tag checkout content = %q, want the tagged %q", content, "version one")
	}
}

// A commit SHA must resolve even though it is neither branch nor tag.
func TestGitProvider_CommitShaRef(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	firstSHA := runGit(t, upstream, "rev-parse", "HEAD")

	if err := os.WriteFile(filepath.Join(upstream, "skills", "demo", "SKILL.md"), []byte("later"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "--quiet", "-m", "later")

	cache := t.TempDir()
	p := &GitProvider{}

	res, err := p.Fetch(types.SkillDependency{
		Name:   "demo",
		Source: upstream,
		Path:   "skills/demo",
		Ref:    firstSHA,
	}, cache, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch failed: %v", err)
	}

	if res.CommitSHA != firstSHA {
		t.Errorf("CommitSHA = %q, want %q", res.CommitSHA, firstSHA)
	}
}

// An empty ref means "the default branch", and it must keep tracking it.
func TestGitProvider_EmptyRefTracksDefaultBranch(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()

	skill := types.SkillDependency{
		Name:   "demo",
		Source: upstream,
		Path:   "skills/demo",
	}

	p := &GitProvider{}
	if _, err := p.Fetch(skill, cache, FetchOptions{}); err != nil {
		t.Fatalf("first Fetch failed: %v", err)
	}

	if err := os.WriteFile(filepath.Join(upstream, "skills", "demo", "SKILL.md"), []byte("moved on"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "--quiet", "-m", "moved on")
	wantSHA := runGit(t, upstream, "rev-parse", "HEAD")

	second, err := p.Fetch(skill, cache, FetchOptions{})
	if err != nil {
		t.Fatalf("second Fetch failed: %v", err)
	}

	if second.CommitSHA != wantSHA {
		t.Errorf("CommitSHA = %q, want %q; the default branch did not advance", second.CommitSHA, wantSHA)
	}
}

func TestGitProvider_UnknownRefErrors(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()

	p := &GitProvider{}
	_, err := p.Fetch(types.SkillDependency{
		Name:   "demo",
		Source: upstream,
		Path:   "skills/demo",
		Ref:    "no-such-branch",
	}, cache, FetchOptions{})

	if err == nil {
		t.Fatal("Fetch() accepted a ref that does not exist, want error")
	}
	if !strings.Contains(err.Error(), "no-such-branch") {
		t.Errorf("error = %q, want it to name the missing ref", err.Error())
	}
}
