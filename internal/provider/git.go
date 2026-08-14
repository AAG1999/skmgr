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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
)

// GitProvider fetches skills from remote git repositories.
type GitProvider struct{}

// Fetch implements Provider for git sources.
//
// The tree is materialized into a per-commit directory in the cache, which is
// immutable once written. Everything below therefore reads a private snapshot
// rather than a shared checkout that another skmgr process might move.
func (p *GitProvider) Fetch(skill types.SkillDependency, cacheDir string, opts FetchOptions) (FetchResult, error) {
	// Normalize once, so the cache key and every git invocation below see the
	// URL git understands rather than skmgr's git:: notation.
	source := CloneURL(skill.Source)

	cache := newGitCache(cacheDir, source)

	materialized, err := cache.materialize(source, skill.Ref, opts)
	if err != nil {
		return FetchResult{}, err
	}

	ref := skill.Ref
	switch {
	case materialized.ResolvedRef != "":
		ref = materialized.ResolvedRef
	case ref == "":
		ref = "the default branch"
	}

	repoDir := materialized.Dir

	var sourceDirs []string

	if skill.Path != "" && !strings.ContainsAny(skill.Path, "*?[") {
		// Direct explicit path
		dir, err := resolveWithinBase(repoDir, skill.Path)
		if err != nil {
			return FetchResult{}, err
		}
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return FetchResult{}, fmt.Errorf("path %q not found in repository at ref %q", skill.Path, ref)
		}
		sourceDirs = []string{dir}
	} else {
		// Wildcard path OR Auto-discovery
		if skill.Path != "" {
			// Wildcard
			globPattern := filepath.Join(repoDir, filepath.FromSlash(skill.Path))
			matches, err := filepath.Glob(globPattern)
			if err != nil {
				return FetchResult{}, fmt.Errorf("invalid glob pattern %q: %w", skill.Path, err)
			}

			matches, err = filterWithinBase(repoDir, matches)
			if err != nil {
				return FetchResult{}, err
			}

			// The user named this pattern, so a match is taken wherever it sits
			// in the tree rather than only under skills/ or rules/.
			collector := &skillCollector{}
			collector.considerMatches(matches)
			sourceDirs = collector.dirs
		} else {
			// Auto-discovery. Nothing points at a particular directory here, so
			// the skills/ and rules/ convention is what separates a real skill
			// from a stray marker file.
			collector := &skillCollector{requireConventionalParent: true}
			if err := collector.walk(repoDir); err != nil {
				return FetchResult{}, fmt.Errorf("auto-discovery failed: %w", err)
			}
			sourceDirs = collector.dirs
		}

		if len(sourceDirs) == 0 {
			if skill.Path != "" {
				return FetchResult{}, fmt.Errorf("no valid skills found matching path %q in repository at ref %q", skill.Path, ref)
			}

			dirs, err := rootFallback(repoDir, fmt.Sprintf("%s at ref %q", skill.Source, ref))
			if err != nil {
				return FetchResult{}, err
			}
			sourceDirs = dirs
		}
	}

	return FetchResult{
		SourceDirs:  sourceDirs,
		CommitSHA:   materialized.SHA,
		ResolvedRef: materialized.ResolvedRef,
	}, nil
}

// GitInstalled checks if the git executable is available in PATH.
func GitInstalled() bool {
	_, err := exec.LookPath("git")
	return err == nil
}
