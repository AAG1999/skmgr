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

package engine

import (
	"fmt"
	"sort"

	"github.com/AAG1999/skmgr/internal/types"
)

// PlanAction is what would happen to one skill.
type PlanAction string

const (
	// PlanInstall means the skill is not installed yet.
	PlanInstall PlanAction = "install"

	// PlanUpdate means the skill is installed but would move to a different
	// commit or a different version.
	PlanUpdate PlanAction = "update"

	// PlanReinstall means the resolved commit is unchanged but what is on disk
	// does not match it, so the files would be rewritten.
	PlanReinstall PlanAction = "reinstall"

	// PlanUnchanged means nothing would happen.
	PlanUnchanged PlanAction = "unchanged"

	// PlanRemove means the skill is installed but no longer declared, so it
	// would be deleted.
	PlanRemove PlanAction = "remove"
)

// PlanEntry describes the fate of a single skill.
type PlanEntry struct {
	Name   string     `json:"name"`
	Action PlanAction `json:"action"`
	Type   string     `json:"type,omitempty"`
	Scope  string     `json:"scope,omitempty"`
	Source string     `json:"source,omitempty"`
	Path   string     `json:"path,omitempty"`

	// FromRef and ToRef are the human-readable versions, which for a semver
	// range are the resolved tags rather than the range itself.
	FromRef string `json:"from_ref,omitempty"`
	ToRef   string `json:"to_ref,omitempty"`

	// FromSHA and ToSHA are the commits. A local source has neither.
	FromSHA string `json:"from_sha,omitempty"`
	ToSHA   string `json:"to_sha,omitempty"`

	// Targets are the agents the skill would be linked into.
	Targets []string `json:"targets,omitempty"`
}

// Changed reports whether an entry represents work.
func (p PlanEntry) Changed() bool {
	return p.Action != PlanUnchanged
}

// Plan is the full set of changes a sync would make.
type Plan struct {
	Entries []PlanEntry `json:"entries"`
}

// Changes returns only the entries that would result in work.
func (p *Plan) Changes() []PlanEntry {
	changed := make([]PlanEntry, 0, len(p.Entries))
	for _, entry := range p.Entries {
		if entry.Changed() {
			changed = append(changed, entry)
		}
	}
	return changed
}

// Empty reports whether the project is already in the requested state.
func (p *Plan) Empty() bool {
	return len(p.Changes()) == 0
}

// Plan works out what Sync would do, without touching the project.
//
// Resolution still runs, because there is no way to say whether a branch has
// moved without asking; that only writes to the download cache, never to the
// project or the lockfile. Nothing is installed, linked, or deleted.
func (e *Engine) Plan(manifest *types.Manifest, existingLock *types.Lockfile, opts SyncOptions) (*Plan, error) {
	if opts.Frozen && existingLock == nil {
		return nil, fmt.Errorf("frozen mode requested but no lockfile exists")
	}

	resolveOpts, err := e.resolveOptionsFor(manifest, existingLock, opts)
	if err != nil {
		return nil, err
	}

	resolved, err := e.Resolver.ResolveWithOptions(manifest.Skills, resolveOpts)
	if err != nil {
		return nil, fmt.Errorf("resolution failed: %w", err)
	}

	// Verification says whether what is on disk still matches the lock, which
	// is what separates "already installed" from "would be rewritten".
	verified := make(map[string]VerifyResult)
	for _, result := range e.Verify(existingLock) {
		verified[result.Name] = result
	}

	plan := &Plan{}
	declared := make(map[string]bool, len(resolved))

	for _, rs := range resolved {
		name := rs.SkillDependency.Name
		declared[name] = true

		entry := PlanEntry{
			Name:    name,
			Type:    rs.SkillDependency.EffectiveType(),
			Scope:   rs.SkillDependency.EffectiveScope(),
			Source:  rs.SkillDependency.Source,
			Path:    rs.SkillDependency.Path,
			ToRef:   displayRef(rs.ResolvedRef, rs.RequestedRef),
			ToSHA:   rs.CommitSHA,
			Targets: rs.SkillDependency.EffectiveTargets(manifest.Targets),
		}

		locked := findLocked(existingLock, name)
		if locked != nil {
			entry.FromRef = displayRef(locked.ResolvedRef, locked.Ref)
			entry.FromSHA = locked.CommitSHA
		}

		entry.Action = planAction(locked, verified[name], rs)
		plan.Entries = append(plan.Entries, entry)
	}

	plan.Entries = append(plan.Entries, removals(existingLock, declared, verified)...)

	sort.SliceStable(plan.Entries, func(i, j int) bool {
		return plan.Entries[i].Name < plan.Entries[j].Name
	})

	return plan, nil
}

// planAction classifies one skill.
func planAction(locked *types.LockEntry, verified VerifyResult, rs ResolvedSkill) PlanAction {
	if locked == nil {
		return PlanInstall
	}

	// A local source has no commit to compare, so the files themselves are the
	// only evidence of a change.
	if rs.CommitSHA != "" && locked.CommitSHA != "" && rs.CommitSHA != locked.CommitSHA {
		return PlanUpdate
	}

	switch verified.Status {
	case VerifyOK:
		return PlanUnchanged
	case VerifyMissing:
		return PlanInstall
	default:
		// Modified, or a lock entry with no hash to check against. Either way
		// the install would be rewritten from the source.
		return PlanReinstall
	}
}

// removals lists installed skills that the manifest no longer declares.
func removals(lock *types.Lockfile, declared map[string]bool, verified map[string]VerifyResult) []PlanEntry {
	if lock == nil {
		return nil
	}

	var entries []PlanEntry
	for _, locked := range lock.Entries {
		if declared[locked.Name] {
			continue
		}
		// Something already gone needs no removing, and reporting it would
		// suggest the dry run found work that does not exist.
		if result, ok := verified[locked.Name]; ok && result.Status == VerifyMissing {
			continue
		}

		entries = append(entries, PlanEntry{
			Name:    locked.Name,
			Action:  PlanRemove,
			Source:  locked.Source,
			Path:    locked.Path,
			FromRef: displayRef(locked.ResolvedRef, locked.Ref),
			FromSHA: locked.CommitSHA,
		})
	}

	return entries
}

// findLocked returns the lock entry for a name, if there is one.
func findLocked(lock *types.Lockfile, name string) *types.LockEntry {
	if lock == nil {
		return nil
	}
	return lock.FindEntry(name)
}

// displayRef prefers the concrete tag a range resolved to, since "^1.2.0" does
// not tell the reader which version they are on.
func displayRef(resolved string, requested string) string {
	if resolved != "" {
		return resolved
	}
	return requested
}

// resolveOptionsFor builds the resolver options for a sync or a plan, so that a
// dry run resolves under exactly the rules the real run would use.
func (e *Engine) resolveOptionsFor(
	manifest *types.Manifest,
	existingLock *types.Lockfile,
	opts SyncOptions,
) (ResolveOptions, error) {
	resolveOpts := ResolveOptions{Offline: opts.Offline}

	switch {
	case opts.Frozen:
		resolveOpts.Pinned = pinnedRefs(manifest, existingLock, nil)
		// A frozen install reproduces the lockfile, so there is nothing to
		// learn from a remote. Staying local also makes CI runs deterministic.
		resolveOpts.Offline = true

	case len(opts.Only) > 0:
		// Everything still has to be resolved, otherwise the unnamed skills
		// would look like orphans and be deleted. Holding them at their locked
		// SHA is what limits the update to the requested skills.
		selected, err := FilterSkills(manifest, existingLock, opts.Only)
		if err != nil {
			return ResolveOptions{}, err
		}
		resolveOpts.Pinned = pinnedRefs(manifest, existingLock, selected)
	}

	// --latest only applies to skills that are actually moving, so a named
	// update does not quietly drag the pinned ones across a major version.
	if opts.Latest {
		resolveOpts.Latest = latestTargets(manifest, resolveOpts.Pinned)
	}

	return resolveOpts, nil
}
