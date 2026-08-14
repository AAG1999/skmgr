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

// Package engine orchestrates the lifecycle of fetching, linking, and locking skills.
package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/provider"
	"github.com/AAG1999/skmgr/internal/types"
)

// Resolver handles fetching and resolving concrete SHAs for skills.
type Resolver struct {
	cacheDir string
}

// NewResolver creates a new Resolver.
func NewResolver(cacheDir string) *Resolver {
	return &Resolver{
		cacheDir: cacheDir,
	}
}

// ResolvedSkill contains the fetched source directory and commit SHA.
type ResolvedSkill struct {
	SkillDependency types.SkillDependency
	SourceDir       string
	CommitSHA       string
	ContentHash     string // Calculated later by installer

	// RequestedRef is what the manifest asked for, kept separate from the
	// dependency's Ref because pinning overwrites that with a commit SHA.
	RequestedRef string

	// ResolvedRef is the tag a version range selected, empty otherwise.
	ResolvedRef string
}

// ResolveOptions controls how dependencies are resolved.
type ResolveOptions struct {
	// Pinned maps a manifest entry to the commit SHA recorded in the lockfile.
	// When a skill has an entry here, that SHA replaces the manifest's ref so
	// a frozen install reproduces exactly what was locked.
	Pinned map[string]string

	// Offline forbids network access during fetch.
	Offline bool

	// Latest ignores declared version ranges and takes the newest release tag.
	Latest map[string]bool
}

// pinKey identifies a manifest entry independently of its local alias, so a
// wildcard skill (whose names are generated per match) can still be pinned.
func pinKey(skill types.SkillDependency) string {
	return skill.Source + "\x00" + skill.Path
}

// describeSource names a dependency for a progress line, preferring the local
// alias and falling back to the source when a wildcard entry has no name yet.
func describeSource(skill types.SkillDependency) string {
	label := skill.Name
	if label == "" {
		label = skill.Source
		if skill.Path != "" {
			label = fmt.Sprintf("%s (%s)", skill.Source, skill.Path)
		}
	}

	if skill.Ref != "" {
		return fmt.Sprintf("%s at %s", label, skill.Ref)
	}
	return label
}

// Resolve fetches each dependency and returns its resolved location and SHA.
// Name conflicts are detected as names are assigned.
func (r *Resolver) Resolve(skills []types.SkillDependency) ([]ResolvedSkill, error) {
	return r.ResolveWithOptions(skills, ResolveOptions{})
}

// ResolveWithOptions is Resolve with pinning and offline control.
func (r *Resolver) ResolveWithOptions(skills []types.SkillDependency, opts ResolveOptions) ([]ResolvedSkill, error) {
	var resolved []ResolvedSkill
	seen := make(map[string]bool)

	for index, skill := range skills {
		requestedRef := skill.Ref

		// A frozen install must check out the locked commit, not whatever the
		// manifest's ref currently points at.
		if sha, ok := opts.Pinned[pinKey(skill)]; ok && sha != "" {
			skill.Ref = sha
		}

		// Fetching a large repository takes long enough that silence looks like
		// a hang, so each source is announced before the work starts.
		log.Infof("[%d/%d] Fetching %s", index+1, len(skills), describeSource(skill))

		// Get appropriate provider
		prov := provider.GetProvider(skill.Source)

		// Let the provider handle the path validation
		res, err := prov.Fetch(skill, r.cacheDir, provider.FetchOptions{
			Offline: opts.Offline,
			Latest:  opts.Latest[pinKey(skill)],
		})
		if err != nil {
			// Check if it's a path not found error from git provider
			if strings.Contains(err.Error(), "not found in repository") {
				return nil, fmt.Errorf("invalid path for skill %q: %w", skill.Name, err)
			}
			return nil, fmt.Errorf("failed to fetch skill %q: %w", skill.Name, err)
		}

		for _, dir := range res.SourceDirs {
			finalName := skill.Name

			isAutoDiscover := strings.ContainsAny(skill.Path, "*?[") || skill.Path == ""

			// If name is empty, or if this fetch yielded multiple directories (meaning a wildcard or auto-discovery was used),
			// we must generate a unique name for each directory.
			if finalName == "" || (isAutoDiscover && len(res.SourceDirs) > 1) {
				base := filepath.Base(dir)
				if skill.Name != "" {
					finalName = fmt.Sprintf("%s-%s", skill.Name, base)
				} else {
					finalName = base
				}
			}

			if seen[finalName] {
				return nil, fmt.Errorf("duplicate skill name resolved: %q", finalName)
			}
			seen[finalName] = true

			resolvedDep := skill
			resolvedDep.Name = finalName

			// Auto-detect type if auto-discovery was used (meaning we might be mixing rules and skills).
			// If both or neither are present, we fall back to whatever was specified in the config.
			if isAutoDiscover {
				hasSkill := false
				if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
					hasSkill = true
				}
				hasRule := false
				if _, err := os.Stat(filepath.Join(dir, "AGENTS.md")); err == nil {
					hasRule = true
				}

				if hasRule && !hasSkill {
					resolvedDep.Type = types.TypeRule
				} else if hasSkill && !hasRule {
					resolvedDep.Type = types.TypeSkill
				}
			}

			resolved = append(resolved, ResolvedSkill{
				SkillDependency: resolvedDep,
				SourceDir:       dir,
				CommitSHA:       res.CommitSHA,
				RequestedRef:    requestedRef,
				ResolvedRef:     res.ResolvedRef,
			})
		}
	}

	return resolved, nil
}
