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
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
)

// LocalProvider fetches skills from a local file path.
type LocalProvider struct{}

// Fetch implements Provider for local sources.
// Local sources need no network, so FetchOptions.Offline has no effect here.
func (p *LocalProvider) Fetch(skill types.SkillDependency, cacheDir string, opts FetchOptions) (FetchResult, error) {
	// Strip file:// prefix if present
	source := strings.TrimPrefix(skill.Source, "file://")

	basePath, err := filepath.Abs(source)
	if err != nil {
		return FetchResult{}, fmt.Errorf("resolving local path: %w", err)
	}

	var sourceDirs []string

	if skill.Path != "" && !strings.ContainsAny(skill.Path, "*?[") {
		dir, err := resolveWithinBase(basePath, skill.Path)
		if err != nil {
			return FetchResult{}, err
		}
		if info, err := os.Stat(dir); err != nil || !info.IsDir() {
			return FetchResult{}, fmt.Errorf("local path %q is not a valid directory", dir)
		}
		sourceDirs = []string{dir}
	} else {
		if skill.Path != "" {
			globPattern := filepath.Join(basePath, filepath.FromSlash(skill.Path))
			matches, err := filepath.Glob(globPattern)
			if err != nil {
				return FetchResult{}, fmt.Errorf("invalid glob pattern %q: %w", skill.Path, err)
			}

			matches, err = filterWithinBase(basePath, matches)
			if err != nil {
				return FetchResult{}, err
			}

			// The user named this pattern, so a match is taken wherever it sits
			// in the tree rather than only under skills/ or rules/.
			collector := &skillCollector{}
			collector.considerMatches(matches)
			sourceDirs = collector.dirs
		} else {
			if info, err := os.Stat(basePath); err != nil || !info.IsDir() {
				return FetchResult{}, fmt.Errorf("local source %q is not a valid directory", basePath)
			}

			// Auto-discovery. Nothing points at a particular directory here, so
			// the skills/ and rules/ convention is what separates a real skill
			// from a stray marker file.
			collector := &skillCollector{requireConventionalParent: true}
			if err := collector.walk(basePath); err != nil {
				return FetchResult{}, fmt.Errorf("auto-discovery failed: %w", err)
			}
			sourceDirs = collector.dirs
		}

		if len(sourceDirs) == 0 {
			if skill.Path != "" {
				return FetchResult{}, fmt.Errorf("no valid skills found matching path %q in local source %q", skill.Path, basePath)
			}

			dirs, err := rootFallback(basePath, fmt.Sprintf("%q", basePath))
			if err != nil {
				return FetchResult{}, err
			}
			sourceDirs = dirs
		}
	}

	// For local providers, we don't copy to cache. We just use the path directly.
	// We also don't have a CommitSHA.
	return FetchResult{
		SourceDirs: sourceDirs,
		CommitSHA:  "",
	}, nil
}
