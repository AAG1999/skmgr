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
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// CacheEntry describes one cached commit tree.
//
// Pruning works at commit granularity rather than per repository: a repository
// followed on a moving branch accumulates one tree per commit it ever saw, and
// those old trees are the bulk of what grows without bound.
type CacheEntry struct {
	// Path is the directory holding the extracted tree.
	Path string

	// Repo is the source-hash directory the commit belongs to.
	Repo string

	// SHA is the commit the tree was extracted from.
	SHA string

	// Size is the tree's total size in bytes.
	Size int64

	// LastUsed is the most recent access or modification time observed.
	LastUsed time.Time
}

// CacheRepo describes one cached source repository: its clone plus the commit
// trees extracted from it.
//
// The clone is tracked separately because it is often the larger half of the
// cache, and because it only becomes removable once every tree extracted from
// it has been pruned.
type CacheRepo struct {
	// Path is the repository's directory under <cacheDir>/git.
	Path string

	// Name is the hash of the source URL that keys the directory.
	Name string

	// CloneSize is the size of everything outside the commit trees, which in
	// practice means the clone.
	CloneSize int64

	// Commits are the extracted trees, newest first.
	Commits []CacheEntry
}

// ScanCache lists every cached commit tree, newest first.
func ScanCache(cacheDir string) ([]CacheEntry, error) {
	repos, err := ScanRepos(cacheDir)
	if err != nil {
		return nil, err
	}

	var entries []CacheEntry
	for _, repo := range repos {
		entries = append(entries, repo.Commits...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})

	return entries, nil
}

// ScanRepos lists cached repositories with their clones and commit trees.
func ScanRepos(cacheDir string) ([]CacheRepo, error) {
	gitRoot := filepath.Join(cacheDir, "git")

	dirs, err := os.ReadDir(gitRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var repos []CacheRepo

	for _, dir := range dirs {
		if !dir.IsDir() {
			continue
		}

		repoPath := filepath.Join(gitRoot, dir.Name())
		repo := CacheRepo{Path: repoPath, Name: dir.Name()}

		commitsDir := filepath.Join(repoPath, "commits")
		commits, err := os.ReadDir(commitsDir)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}

		var commitBytes int64

		for _, commit := range commits {
			if !commit.IsDir() {
				continue
			}

			path := filepath.Join(commitsDir, commit.Name())
			size, modified, err := treeStats(path)
			if err != nil {
				return nil, err
			}

			commitBytes += size
			repo.Commits = append(repo.Commits, CacheEntry{
				Path:     path,
				Repo:     dir.Name(),
				SHA:      commit.Name(),
				Size:     size,
				LastUsed: modified,
			})
		}

		total, _, err := treeStats(repoPath)
		if err != nil {
			return nil, err
		}
		repo.CloneSize = total - commitBytes

		sort.Slice(repo.Commits, func(i, j int) bool {
			return repo.Commits[i].LastUsed.After(repo.Commits[j].LastUsed)
		})

		repos = append(repos, repo)
	}

	return repos, nil
}

// treeStats returns a directory's total size and most recent modification time.
func treeStats(root string) (int64, time.Time, error) {
	var size int64
	var newest time.Time

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			// A file removed mid-walk is not worth failing the scan over.
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !d.IsDir() {
			size += info.Size()
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return 0, time.Time{}, err
	}

	return size, newest, nil
}

// PruneOptions selects which cache entries to remove.
type PruneOptions struct {
	// OlderThan removes entries not touched within this duration. Zero means
	// age is not considered.
	OlderThan time.Duration

	// MaxSize keeps the cache under this many bytes, removing the least
	// recently used entries first. Zero means size is not considered.
	MaxSize int64

	// DryRun reports what would be removed without removing it.
	DryRun bool
}

// PruneResult summarizes a prune.
type PruneResult struct {
	// Removed lists the commit trees that were dropped.
	Removed []CacheEntry

	// RemovedRepos counts repositories whose clone was dropped as well,
	// because none of their trees survived.
	RemovedRepos int

	// FreedSize counts the trees plus any clones that went with them.
	FreedSize int64

	// Remaining is what the cache still occupies, clones included.
	Remaining int64
}

// PruneCache removes cached commit trees according to opts.
//
// A repository that loses every one of its trees loses its clone too. Keeping
// the clone would mean "prune" reclaimed only part of the space and left the
// cache reporting itself as empty while still holding the larger half.
//
// With no options set, everything is removed: "prune with no criteria" is the
// same request as "empty the cache", and refusing would just make the user run
// a different command.
func PruneCache(cacheDir string, opts PruneOptions) (PruneResult, error) {
	repos, err := ScanRepos(cacheDir)
	if err != nil {
		return PruneResult{}, err
	}

	var result PruneResult
	removeAll := opts.OlderThan == 0 && opts.MaxSize == 0
	cutoff := time.Now().Add(-opts.OlderThan)

	// The size budget spans the whole cache, so it has to be applied across
	// repositories rather than within each one.
	var kept int64

	for _, entry := range flattenNewestFirst(repos) {
		drop := removeAll

		if opts.OlderThan > 0 && entry.LastUsed.Before(cutoff) {
			drop = true
		}

		// Entries are newest first, so this keeps the most recently used ones
		// and drops the tail once the budget is spent.
		if !drop && opts.MaxSize > 0 && kept+entry.Size > opts.MaxSize {
			drop = true
		}

		if !drop {
			kept += entry.Size
			continue
		}

		if !opts.DryRun {
			if err := os.RemoveAll(entry.Path); err != nil {
				return result, err
			}
		}
		result.Removed = append(result.Removed, entry)
		result.FreedSize += entry.Size
	}

	dropped := make(map[string]bool, len(result.Removed))
	for _, entry := range result.Removed {
		dropped[entry.Path] = true
	}

	for _, repo := range repos {
		if !cloneIsRemovable(repo, dropped, removeAll) {
			result.Remaining += repo.CloneSize
			continue
		}

		if !opts.DryRun {
			if err := os.RemoveAll(repo.Path); err != nil {
				return result, err
			}
		}
		result.RemovedRepos++
		result.FreedSize += repo.CloneSize
	}

	result.Remaining += kept
	return result, nil
}

// flattenNewestFirst orders every commit tree in the cache by last use.
func flattenNewestFirst(repos []CacheRepo) []CacheEntry {
	var entries []CacheEntry
	for _, repo := range repos {
		entries = append(entries, repo.Commits...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].LastUsed.After(entries[j].LastUsed)
	})

	return entries
}

// cloneIsRemovable reports whether a repository's clone should go too.
//
// A clone is only dropped once every tree extracted from it has been, since a
// surviving tree implies the repository is still in use. A repository holding a
// clone and no trees is left alone unless the request was to empty the cache
// outright: it is most likely a clone from a run that is still in progress, and
// an age or size filter said nothing about it.
func cloneIsRemovable(repo CacheRepo, dropped map[string]bool, removeAll bool) bool {
	if len(repo.Commits) == 0 {
		return removeAll
	}

	for _, commit := range repo.Commits {
		if !dropped[commit.Path] {
			return false
		}
	}

	return true
}
