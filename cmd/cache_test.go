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

package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

// seedCacheEntry creates a fake cached commit tree of roughly the given size.
func seedCacheEntry(t *testing.T, repo string, sha string, size int, age time.Duration) string {
	t.Helper()

	dir := filepath.Join(types.CacheDir(), "git", repo, "commits", sha)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	file := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(file, bytes.Repeat([]byte("x"), size), 0644); err != nil {
		t.Fatal(err)
	}

	when := time.Now().Add(-age)
	if err := os.Chtimes(file, when, when); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(dir, when, when); err != nil {
		t.Fatal(err)
	}

	return dir
}

// seedCacheClone creates the clone half of a cached repository, which is what
// survives pruning a single commit tree and must not survive pruning them all.
func seedCacheClone(t *testing.T, repo string, size int) string {
	t.Helper()

	dir := filepath.Join(types.CacheDir(), "git", repo, "repo")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(dir, "HEAD"), bytes.Repeat([]byte("x"), size), 0644); err != nil {
		t.Fatal(err)
	}

	return filepath.Join(types.CacheDir(), "git", repo)
}

func runCache(t *testing.T, cmdToRun *cobra.Command, args []string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	cmdToRun.SetOut(&out)
	t.Cleanup(func() { cmdToRun.SetOut(nil) })

	err := cmdToRun.RunE(cmdToRun, args)
	return out.String(), err
}

func TestCachePath_PrintsTheCacheDirectory(t *testing.T) {
	newProjectDir(t)

	out, err := runCache(t, cachePathCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out) != types.CacheDir() {
		t.Errorf("path = %q, want %q", strings.TrimSpace(out), types.CacheDir())
	}
}

func TestCacheInfo_ReportsEmptyAndPopulated(t *testing.T) {
	newProjectDir(t)

	out, err := runCache(t, cacheInfoCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "empty") {
		t.Errorf("expected an empty report, got:\n%s", out)
	}

	seedCacheEntry(t, "repo-a", "sha1", 100, time.Hour)
	seedCacheEntry(t, "repo-b", "sha2", 200, time.Hour)

	out, err = runCache(t, cacheInfoCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "2 repositories") {
		t.Errorf("expected two repositories, got:\n%s", out)
	}
	if !strings.Contains(out, "2 cached commits") {
		t.Errorf("expected two commits, got:\n%s", out)
	}
}

func TestCachePrune_ByAge(t *testing.T) {
	newProjectDir(t)

	old := seedCacheEntry(t, "repo-a", "old", 100, 60*24*time.Hour)
	recent := seedCacheEntry(t, "repo-a", "recent", 100, time.Hour)

	pruneOlderThan = "30d"

	out, err := runCache(t, cachePruneCmd, nil)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Errorf("the stale entry should have been pruned:\n%s", out)
	}
	if _, err := os.Stat(recent); err != nil {
		t.Errorf("the recent entry should have survived: %v", err)
	}
}

func TestCachePrune_BySize(t *testing.T) {
	newProjectDir(t)

	// Newest first is what survives, so age determines who wins the budget.
	oldest := seedCacheEntry(t, "repo-a", "oldest", 2000, 3*time.Hour)
	middle := seedCacheEntry(t, "repo-a", "middle", 2000, 2*time.Hour)
	newest := seedCacheEntry(t, "repo-a", "newest", 2000, time.Hour)

	pruneMaxSize = "4KB"

	if _, err := runCache(t, cachePruneCmd, nil); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(newest); err != nil {
		t.Errorf("the newest entry should be kept: %v", err)
	}
	if _, err := os.Stat(middle); err != nil {
		t.Errorf("the second-newest entry fits in the budget: %v", err)
	}
	if _, err := os.Stat(oldest); !os.IsNotExist(err) {
		t.Error("the least recently used entry should have been dropped")
	}
}

func TestCachePrune_DryRunRemovesNothing(t *testing.T) {
	newProjectDir(t)

	entry := seedCacheEntry(t, "repo-a", "old", 100, 60*24*time.Hour)

	pruneOlderThan = "30d"
	pruneDryRun = true

	out, err := runCache(t, cachePruneCmd, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "Would remove") {
		t.Errorf("dry run should say what it would do, got:\n%s", out)
	}
	if _, err := os.Stat(entry); err != nil {
		t.Error("a dry run must not delete anything")
	}
}

func TestCachePrune_NoCriteriaEmptiesTheCache(t *testing.T) {
	newProjectDir(t)

	entry := seedCacheEntry(t, "repo-a", "any", 100, time.Hour)

	if _, err := runCache(t, cachePruneCmd, nil); err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(entry); !os.IsNotExist(err) {
		t.Error("prune with no criteria should empty the cache")
	}
}

func TestCachePrune_ReportsWhenNothingMatches(t *testing.T) {
	newProjectDir(t)

	seedCacheEntry(t, "repo-a", "recent", 100, time.Hour)
	pruneOlderThan = "30d"

	out, err := runCache(t, cachePruneCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Nothing to prune") {
		t.Errorf("expected a no-op report, got:\n%s", out)
	}
}

// Pruning every tree of a repository must take the clone with it. Leaving it
// behind made "cache is empty" a lie and reclaimed only part of the space.
func TestCachePrune_DropsTheCloneWhenNoTreesSurvive(t *testing.T) {
	newProjectDir(t)

	repo := seedCacheClone(t, "repo-a", 5000)
	seedCacheEntry(t, "repo-a", "old", 100, 60*24*time.Hour)

	pruneOlderThan = "30d"

	out, err := runCache(t, cachePruneCmd, nil)
	if err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(repo); !os.IsNotExist(err) {
		t.Errorf("the clone should have gone with its last tree:\n%s", out)
	}
	if !strings.Contains(out, "clone") {
		t.Errorf("the report should mention the clone, got:\n%s", out)
	}

	out, err = runCache(t, cacheInfoCmd, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "empty") {
		t.Errorf("the cache should now report as empty, got:\n%s", out)
	}
}

func TestCachePrune_KeepsTheCloneWhenATreeSurvives(t *testing.T) {
	newProjectDir(t)

	repo := seedCacheClone(t, "repo-a", 5000)
	seedCacheEntry(t, "repo-a", "old", 100, 60*24*time.Hour)
	recent := seedCacheEntry(t, "repo-a", "recent", 100, time.Hour)

	pruneOlderThan = "30d"

	if _, err := runCache(t, cachePruneCmd, nil); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(recent); err != nil {
		t.Errorf("the recent tree should have survived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, "repo")); err != nil {
		t.Errorf("the clone is still in use and must be kept: %v", err)
	}
}

// An age filter says nothing about a clone with no trees yet, which is what a
// run still in progress looks like.
func TestCachePrune_LeavesAFreshCloneAloneUnderAFilter(t *testing.T) {
	newProjectDir(t)

	repo := seedCacheClone(t, "repo-a", 5000)
	pruneOlderThan = "30d"

	if _, err := runCache(t, cachePruneCmd, nil); err != nil {
		t.Fatalf("prune: %v", err)
	}

	if _, err := os.Stat(repo); err != nil {
		t.Errorf("a filtered prune should not delete a treeless clone: %v", err)
	}

	// Asking to empty the cache is a different request, and does remove it.
	pruneOlderThan = ""
	if _, err := runCache(t, cachePruneCmd, nil); err != nil {
		t.Fatalf("prune: %v", err)
	}
	if _, err := os.Stat(repo); !os.IsNotExist(err) {
		t.Error("prune with no criteria should empty the cache completely")
	}
}

func TestCacheInfo_BreaksOutCloneSize(t *testing.T) {
	newProjectDir(t)

	seedCacheClone(t, "repo-a", 4096)
	seedCacheEntry(t, "repo-a", "sha1", 1024, time.Hour)

	out, err := runCache(t, cacheInfoCmd, nil)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(out, "in clones") {
		t.Errorf("expected the clone total to be broken out, got:\n%s", out)
	}
	if !strings.Contains(out, "1 repository,") {
		t.Errorf("expected a singular repository, got:\n%s", out)
	}
	if !strings.Contains(out, "1 cached commit,") {
		t.Errorf("expected a singular commit, got:\n%s", out)
	}
}

func TestParseDuration(t *testing.T) {
	cases := []struct {
		input string
		want  time.Duration
	}{
		{"48h", 48 * time.Hour},
		{"30d", 30 * 24 * time.Hour},
		{"2w", 14 * 24 * time.Hour},
		{"90m", 90 * time.Minute},
	}

	for _, tc := range cases {
		got, err := parseDuration(tc.input)
		if err != nil {
			t.Errorf("parseDuration(%q): %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}

	if _, err := parseDuration("soon"); err == nil {
		t.Error("expected an error for an unparseable duration")
	}
}

func TestParseSize(t *testing.T) {
	cases := []struct {
		input string
		want  int64
	}{
		{"512", 512},
		{"1KB", 1024},
		{"2MB", 2 * 1024 * 1024},
		{"1GB", 1024 * 1024 * 1024},
		{"1.5MB", 1572864},
	}

	for _, tc := range cases {
		got, err := parseSize(tc.input)
		if err != nil {
			t.Errorf("parseSize(%q): %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("parseSize(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}

	if _, err := parseSize("huge"); err == nil {
		t.Error("expected an error for an unparseable size")
	}
}
