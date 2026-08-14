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
	"strings"
	"sync"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func demoSkill(source string, ref string) types.SkillDependency {
	return types.SkillDependency{
		Name:   "demo",
		Source: source,
		Path:   "skills/demo",
		Ref:    ref,
	}
}

// Offline must work from the cache and fail clearly without it, rather than
// hanging on a network call that cannot succeed.
func TestGitProvider_OfflineRequiresAPopulatedCache(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()
	p := &GitProvider{}

	_, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{Offline: true})
	if err == nil {
		t.Fatal("offline should fail when nothing is cached")
	}
	if !strings.Contains(err.Error(), "offline") {
		t.Errorf("the error should mention offline mode, got: %v", err)
	}

	if _, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{}); err != nil {
		t.Fatalf("populating the cache: %v", err)
	}

	result, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{Offline: true})
	if err != nil {
		t.Fatalf("offline should succeed once the cache is populated: %v", err)
	}
	if len(result.SourceDirs) != 1 {
		t.Fatalf("expected one source dir, got %v", result.SourceDirs)
	}
}

// Offline must not follow a moving branch, because doing so requires a fetch.
func TestGitProvider_OfflineServesTheCachedCommit(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()
	p := &GitProvider{}

	first, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	skillFile := filepath.Join(upstream, "skills", "demo", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("version two"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "--quiet", "-m", "second")

	offline, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{Offline: true})
	if err != nil {
		t.Fatalf("offline fetch: %v", err)
	}
	if offline.CommitSHA != first.CommitSHA {
		t.Error("offline should not have advanced to the new commit")
	}

	online, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if online.CommitSHA == first.CommitSHA {
		t.Error("an online fetch should have picked up the new commit")
	}
}

// Each commit gets its own directory, so two refs of one repository no longer
// fight over a single shared checkout.
func TestGitProvider_DifferentRefsGetSeparateDirectories(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	runGit(t, upstream, "tag", "v1")

	skillFile := filepath.Join(upstream, "skills", "demo", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("version two"), 0644); err != nil {
		t.Fatal(err)
	}
	runGit(t, upstream, "add", ".")
	runGit(t, upstream, "commit", "--quiet", "-m", "second")
	runGit(t, upstream, "tag", "v2")

	cache := t.TempDir()
	p := &GitProvider{}

	one, err := p.Fetch(demoSkill(upstream, "v1"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	two, err := p.Fetch(demoSkill(upstream, "v2"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if one.SourceDirs[0] == two.SourceDirs[0] {
		t.Fatal("two refs resolved to the same cache directory")
	}

	// Fetching v2 must not have disturbed the tree v1 points at.
	body, err := os.ReadFile(filepath.Join(one.SourceDirs[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "version one" {
		t.Errorf("v1 tree = %q, want it untouched by the v2 fetch", body)
	}

	body, err = os.ReadFile(filepath.Join(two.SourceDirs[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "version two" {
		t.Errorf("v2 tree = %q", body)
	}
}

// The cached tree must not carry git metadata into the install.
func TestGitProvider_CachedTreeHasNoGitDirectory(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()
	p := &GitProvider{}

	result, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// Walk up to the commit root, which is the extracted tree.
	commitRoot := filepath.Dir(filepath.Dir(result.SourceDirs[0]))
	if _, err := os.Stat(filepath.Join(commitRoot, ".git")); !os.IsNotExist(err) {
		t.Error("the extracted tree should contain no .git directory")
	}
}

// Concurrent fetches of the same repository must not corrupt the cache. This
// is the everyday case on a CI machine running several jobs at once.
func TestGitProvider_ConcurrentFetchesAreSafe(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	cache := t.TempDir()

	var wg sync.WaitGroup
	results := make([]string, 6)
	errs := make([]error, 6)

	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			p := &GitProvider{}
			res, err := p.Fetch(demoSkill(upstream, "main"), cache, FetchOptions{})
			if err != nil {
				errs[i] = err
				return
			}
			results[i] = res.CommitSHA
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d failed: %v", i, err)
		}
	}

	for i := 1; i < len(results); i++ {
		if results[i] != results[0] {
			t.Errorf("goroutine %d resolved %q, want %q", i, results[i], results[0])
		}
	}
}

// A tag is immutable, so its cached tree should be reused rather than refetched.
func TestGitProvider_ReusesCachedCommitTree(t *testing.T) {
	requireGit(t)

	upstream := newUpstreamRepo(t)
	runGit(t, upstream, "tag", "v1")

	cache := t.TempDir()
	p := &GitProvider{}

	first, err := p.Fetch(demoSkill(upstream, "v1"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	// A marker inside the cached tree proves the second fetch reused it rather
	// than extracting a fresh copy.
	marker := filepath.Join(first.SourceDirs[0], "marker.txt")
	if err := os.WriteFile(marker, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	second, err := p.Fetch(demoSkill(upstream, "v1"), cache, FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if second.SourceDirs[0] != first.SourceDirs[0] {
		t.Fatalf("expected the same cache directory, got %q then %q",
			first.SourceDirs[0], second.SourceDirs[0])
	}
	if _, err := os.Stat(marker); err != nil {
		t.Error("the cached tree was re-extracted instead of reused")
	}
}
