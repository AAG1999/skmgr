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

// newTaggedRepo builds a repository with one commit per tag, each recording its
// own version in the skill so a checkout can be identified.
func newTaggedRepo(t *testing.T, tags ...string) string {
	t.Helper()

	dir := t.TempDir()
	skill := filepath.Join(dir, "skills", "versioned")
	if err := os.MkdirAll(skill, 0755); err != nil {
		t.Fatal(err)
	}

	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	run("init", "--quiet", "-b", "main")

	for _, tag := range tags {
		content := "# versioned skill\n\nversion: " + tag + "\n"
		if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		run("add", ".")
		run("commit", "--quiet", "-m", "release "+tag)
		run("tag", tag)
	}

	return dir
}

// installedVersion reports which tag's content a fetch produced.
func installedVersion(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "version: "); ok {
			return strings.TrimSpace(rest)
		}
	}

	t.Fatalf("no version marker in %s", dir)
	return ""
}

func fetchRef(t *testing.T, source string, cacheDir string, ref string, opts FetchOptions) FetchResult {
	t.Helper()

	p := &GitProvider{}
	res, err := p.Fetch(types.SkillDependency{
		Name:   "versioned",
		Source: source,
		Path:   "skills/versioned",
		Ref:    ref,
	}, cacheDir, opts)
	if err != nil {
		t.Fatalf("fetch %q: %v", ref, err)
	}

	return res
}

func TestGitProvider_ResolvesSemverRanges(t *testing.T) {
	source := newTaggedRepo(t, "v1.0.0", "v1.1.0", "v1.2.0", "v2.0.0")
	cacheDir := t.TempDir()

	cases := []struct {
		ref  string
		want string
	}{
		{ref: "^1.0.0", want: "v1.2.0"},
		{ref: "~1.1.0", want: "v1.1.0"},
		{ref: ">=1.0.0 <2.0.0", want: "v1.2.0"},
		{ref: "1.x", want: "v1.2.0"},
		{ref: "latest", want: "v2.0.0"},
		{ref: "*", want: "v2.0.0"},
	}

	for _, tc := range cases {
		t.Run(tc.ref, func(t *testing.T) {
			res := fetchRef(t, source, cacheDir, tc.ref, FetchOptions{})

			if res.ResolvedRef != tc.want {
				t.Errorf("ResolvedRef = %q, want %q", res.ResolvedRef, tc.want)
			}
			if got := installedVersion(t, res.SourceDirs[0]); got != tc.want {
				t.Errorf("checked out %q, want %q", got, tc.want)
			}
			if res.CommitSHA == "" {
				t.Error("a resolved range should still record a commit")
			}
		})
	}
}

// A literal ref must be used as-is, with nothing recorded as "resolved".
func TestGitProvider_LiteralRefIsNotTreatedAsARange(t *testing.T) {
	source := newTaggedRepo(t, "v1.0.0", "v1.1.0", "v2.0.0")
	cacheDir := t.TempDir()

	res := fetchRef(t, source, cacheDir, "v1.0.0", FetchOptions{})

	if res.ResolvedRef != "" {
		t.Errorf("ResolvedRef = %q, want empty for a literal tag", res.ResolvedRef)
	}
	if got := installedVersion(t, res.SourceDirs[0]); got != "v1.0.0" {
		t.Errorf("checked out %q, want the tag that was named", got)
	}
}

func TestGitProvider_LatestOptionIgnoresTheRange(t *testing.T) {
	source := newTaggedRepo(t, "v1.0.0", "v1.1.0", "v2.0.0")
	cacheDir := t.TempDir()

	res := fetchRef(t, source, cacheDir, "^1.0.0", FetchOptions{Latest: true})

	if res.ResolvedRef != "v2.0.0" {
		t.Errorf("ResolvedRef = %q, want v2.0.0: --latest crosses the range", res.ResolvedRef)
	}
	if got := installedVersion(t, res.SourceDirs[0]); got != "v2.0.0" {
		t.Errorf("checked out %q, want v2.0.0", got)
	}
}

// A new tag on the remote must be picked up, which is the whole point of
// declaring a range rather than a tag.
func TestGitProvider_RangeFollowsNewTags(t *testing.T) {
	source := newTaggedRepo(t, "v1.0.0")
	cacheDir := t.TempDir()

	first := fetchRef(t, source, cacheDir, "^1.0.0", FetchOptions{})
	if first.ResolvedRef != "v1.0.0" {
		t.Fatalf("ResolvedRef = %q, want v1.0.0", first.ResolvedRef)
	}

	// Publish a new patch release.
	skill := filepath.Join(source, "skills", "versioned")
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"),
		[]byte("# versioned skill\n\nversion: v1.0.1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "--quiet", "-m", "release v1.0.1"},
		{"tag", "v1.0.1"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	second := fetchRef(t, source, cacheDir, "^1.0.0", FetchOptions{})
	if second.ResolvedRef != "v1.0.1" {
		t.Errorf("ResolvedRef = %q, want v1.0.1 after a new tag", second.ResolvedRef)
	}
	if got := installedVersion(t, second.SourceDirs[0]); got != "v1.0.1" {
		t.Errorf("checked out %q, want v1.0.1", got)
	}
}

// A repository with no version tags cannot satisfy a range, and the error has
// to say what to do instead.
func TestGitProvider_RangeAgainstAnUntaggedRepo(t *testing.T) {
	source := newTaggedRepo(t)

	// newTaggedRepo with no tags leaves no commits either, so make one.
	skill := filepath.Join(source, "skills", "versioned")
	if err := os.MkdirAll(skill, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# skill\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", "."}, {"commit", "--quiet", "-m", "initial"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = source
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	p := &GitProvider{}
	_, err := p.Fetch(types.SkillDependency{
		Name:   "versioned",
		Source: source,
		Path:   "skills/versioned",
		Ref:    "^1.0.0",
	}, t.TempDir(), FetchOptions{})

	if err == nil {
		t.Fatal("expected an error resolving a range against an untagged repository")
	}
	for _, want := range []string{"no tags", "branch name", "commit SHA"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %v", want, err)
		}
	}
}

func TestListTags(t *testing.T) {
	source := newTaggedRepo(t, "v1.0.0", "v1.1.0", "v2.0.0")

	tags, err := ListTags(source, t.TempDir(), FetchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %v", tags)
	}
	for _, want := range []string{"v1.0.0", "v1.1.0", "v2.0.0"} {
		found := false
		for _, tag := range tags {
			if tag == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing tag %q in %v", want, tags)
		}
	}
}

// Tags come from the clone, so a range resolves without the network once the
// repository is cached.
func TestGitProvider_RangeResolvesOffline(t *testing.T) {
	source := newTaggedRepo(t, "v1.0.0", "v1.1.0")
	cacheDir := t.TempDir()

	fetchRef(t, source, cacheDir, "^1.0.0", FetchOptions{})

	res := fetchRef(t, source, cacheDir, "^1.0.0", FetchOptions{Offline: true})
	if res.ResolvedRef != "v1.1.0" {
		t.Errorf("ResolvedRef = %q, want v1.1.0 from the cache", res.ResolvedRef)
	}
}
