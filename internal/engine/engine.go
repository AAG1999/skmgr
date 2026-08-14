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
	"errors"
	"fmt"
	"time"

	"github.com/AAG1999/skmgr/internal/linker"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/semver"
	"github.com/AAG1999/skmgr/internal/types"
)

// Engine is the central orchestrator for skmgr commands.
type Engine struct {
	ProjectRoot string
	CacheDir    string
	// SkmgrHome holds skmgr's own state, including the global install ledger.
	SkmgrHome string
	Resolver  *Resolver
	Installer *Installer
	Linker    *linker.Linker
}

// NewEngine creates a new Engine.
func NewEngine(projectRoot, cacheDir string) *Engine {
	return &Engine{
		ProjectRoot: projectRoot,
		CacheDir:    cacheDir,
		SkmgrHome:   types.SkmgrHome(),
		Resolver:    NewResolver(cacheDir),
		Installer:   NewInstaller(projectRoot),
		Linker:      linker.NewLinker(),
	}
}

// SyncOptions controls a Sync run.
type SyncOptions struct {
	// Frozen pins every skill to the commit recorded in the lockfile and keeps
	// the resolver from consulting remotes for new refs.
	Frozen bool

	// Offline forbids network access outright.
	Offline bool

	// Only restricts re-resolution to these skill names. Skills outside the
	// list keep their existing lock entry. Empty means "all skills".
	Only []string

	// Latest lets skills leave their declared version range and take the newest
	// release tag. Without it a range is a promise, and `update` keeps it.
	Latest bool
}

// Sync represents the core `skmgr install` flow.
func (e *Engine) Sync(manifest *types.Manifest, existingLock *types.Lockfile, opts SyncOptions) (*types.Lockfile, error) {
	if opts.Frozen && existingLock == nil {
		return nil, fmt.Errorf("frozen mode requested but no lockfile exists")
	}

	resolveOpts, err := e.resolveOptionsFor(manifest, existingLock, opts)
	if err != nil {
		return nil, err
	}

	// 1. Resolve SHAs and fetch sources
	resolvedSkills, err := e.Resolver.ResolveWithOptions(manifest.Skills, resolveOpts)
	if err != nil {
		return nil, fmt.Errorf("resolution failed: %w", err)
	}

	// Rules merged into a shared file (CLAUDE.md and friends) leave no symlink
	// behind, so broken-link pruning cannot find them. Anything present in the
	// old lock but absent now has to be explicitly unlinked before we relink.
	if err := e.removeDroppedEntries(manifest, existingLock, resolvedSkills); err != nil {
		return nil, err
	}

	newLock := &types.Lockfile{
		Version:     types.LockfileVersion,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
	}

	resolvedAt := time.Now().UTC().Format(time.RFC3339)

	// 2. Install skills and link them
	for _, rs := range resolvedSkills {
		// Sweep every agent, not just the current targets. A skill whose target
		// list shrank still has links under the agents it dropped, and those are
		// no longer reachable from the manifest. The correct links are recreated
		// immediately below.
		e.cleanupLinks(rs.SkillDependency.Name, rs.SkillDependency.EffectiveScope(), types.SupportedAgentNames())

		// A frozen install exists to reproduce the lockfile exactly. Pinning the
		// commit is not enough on its own: the cache could be corrupt, and a
		// local source has no commit to pin in the first place.
		expected, err := frozenHash(opts, existingLock, rs.SkillDependency.Name)
		if err != nil {
			return nil, err
		}

		hash, err := e.Installer.InstallExpecting(rs, expected)
		if err != nil {
			// A hash mismatch already explains itself in full, including the
			// skill's name; wrapping it just buries the explanation.
			var mismatch *HashMismatchError
			if errors.As(err, &mismatch) {
				return nil, err
			}
			return nil, fmt.Errorf("install failed for %q: %w", rs.SkillDependency.Name, err)
		}
		rs.ContentHash = hash

		// Create symlinks/merges
		targets := rs.SkillDependency.EffectiveTargets(manifest.Targets)
		for _, target := range targets {
			if rs.SkillDependency.EffectiveType() == types.TypeSkill {
				if err := e.Linker.LinkSkill(target, rs.SkillDependency.Name, rs.SkillDependency.EffectiveScope(), e.ProjectRoot); err != nil {
					return nil, fmt.Errorf("linking skill %q for %q: %w", rs.SkillDependency.Name, target, err)
				}
			} else {
				if err := e.Linker.LinkRule(target, rs.SkillDependency.Name, rs.SkillDependency.EffectiveScope(), e.ProjectRoot); err != nil {
					return nil, fmt.Errorf("linking rule %q for %q: %w", rs.SkillDependency.Name, target, err)
				}
			}
		}

		// Add to lockfile. Source and Path are recorded so a later run can match
		// a manifest entry to its lock entry without relying on the local alias.
		newLock.SetEntry(types.LockEntry{
			Name:        rs.SkillDependency.Name,
			Source:      rs.SkillDependency.Source,
			Path:        rs.SkillDependency.Path,
			Ref:         rs.RequestedRef,
			ResolvedRef: rs.ResolvedRef,
			CommitSHA:   rs.CommitSHA,
			ContentHash: rs.ContentHash,
			ResolvedAt:  resolvedAt,
		})
	}

	// 3. Clean up orphans (skills in .agents/ that are no longer in the manifest)
	if err := e.Installer.CleanOrphans(resolvedSkills, types.ScopeProject); err != nil {
		return nil, fmt.Errorf("cleaning project orphans: %w", err)
	}
	if err := e.cleanGlobalOrphans(resolvedSkills); err != nil {
		return nil, err
	}

	if err := e.Linker.CleanBrokenLinks(types.ScopeProject, e.ProjectRoot); err != nil {
		return nil, fmt.Errorf("cleaning broken project links: %w", err)
	}
	if err := e.Linker.CleanBrokenLinks(types.ScopeGlobal, e.ProjectRoot); err != nil {
		return nil, fmt.Errorf("cleaning broken global links: %w", err)
	}

	return newLock, nil
}

// frozenHash returns the content hash a skill must match, or "" when there is
// nothing to enforce. Only a frozen install enforces anything.
func frozenHash(opts SyncOptions, lock *types.Lockfile, name string) (string, error) {
	if !opts.Frozen {
		return "", nil
	}

	entry := lock.FindEntry(name)
	if entry == nil {
		return "", fmt.Errorf(
			"frozen install: %q is in the manifest but not in the lockfile.\n"+
				"Run 'skmgr install' without --frozen to record it", name)
	}

	// Entries written before content hashing cannot be checked. Refusing would
	// break upgrades for no gain; the next write records a hash.
	return entry.ContentHash, nil
}

// pinnedRefs maps manifest entries to the commit SHA recorded for them in the
// lockfile, keyed by source and path rather than by name. Wildcard entries
// expand into several lock entries that all share one checkout, so matching on
// source and path lets them be pinned even though their names were generated.
//
// Entries in exclude are left unpinned so they re-resolve normally. That is how
// `skmgr update <name>` moves one skill while holding every other one still.
func pinnedRefs(manifest *types.Manifest, lock *types.Lockfile, exclude []types.SkillDependency) map[string]string {
	if lock == nil {
		return nil
	}

	skip := make(map[string]bool, len(exclude))
	for _, skill := range exclude {
		skip[pinKey(skill)] = true
	}

	bySourcePath := make(map[string]string)
	for _, entry := range lock.Entries {
		if entry.CommitSHA == "" {
			continue
		}
		bySourcePath[entry.Source+"\x00"+entry.Path] = entry.CommitSHA
	}

	pinned := make(map[string]string)
	for _, skill := range manifest.Skills {
		key := pinKey(skill)
		if skip[key] {
			continue
		}
		if sha, ok := bySourcePath[key]; ok {
			pinned[key] = sha
		}
	}

	return pinned
}

// latestTargets marks the skills that --latest should apply to: those declaring
// a version range and not held at a pinned commit.
//
// A skill pinned by --frozen or by a named update is deliberately not moving,
// and --latest is not a reason to override that.
func latestTargets(manifest *types.Manifest, pinned map[string]string) map[string]bool {
	targets := make(map[string]bool)

	for _, skill := range manifest.Skills {
		key := pinKey(skill)
		if pinned[key] != "" {
			continue
		}
		if semver.IsRange(skill.Ref) {
			targets[key] = true
		}
	}

	return targets
}

// FilterSkills returns the manifest skills whose name appears in names.
// An unknown name is an error so `skmgr update typo` does not quietly no-op.
//
// Wildcard entries have no name until they are resolved, so a name is also
// matched against the lockfile to find which manifest entry produced it.
func FilterSkills(manifest *types.Manifest, lock *types.Lockfile, names []string) ([]types.SkillDependency, error) {
	if len(names) == 0 {
		return manifest.Skills, nil
	}

	wanted := make(map[string]bool, len(names))
	for _, n := range names {
		wanted[n] = true
	}

	var selected []types.SkillDependency
	matched := make(map[string]bool)

	for _, skill := range manifest.Skills {
		if wanted[skill.Name] {
			selected = append(selected, skill)
			matched[skill.Name] = true
			continue
		}

		// A wildcard entry owns every lock entry sharing its source and path.
		if skill.Name == "" && lock != nil {
			for _, entry := range lock.Entries {
				if wanted[entry.Name] && entry.Source == skill.Source && entry.Path == skill.Path {
					selected = append(selected, skill)
					matched[entry.Name] = true
					break
				}
			}
		}
	}

	for _, n := range names {
		if !matched[n] {
			return nil, fmt.Errorf("no skill named %q in the manifest", n)
		}
	}

	return selected, nil
}

// removeDroppedEntries unlinks anything that was installed previously but is no
// longer part of the resolved set. Symlinked installs would eventually be swept
// by the broken-link pass, but merge-strategy rules are written into a shared
// markdown file and can only be removed by name.
func (e *Engine) removeDroppedEntries(
	manifest *types.Manifest,
	existingLock *types.Lockfile,
	resolved []ResolvedSkill,
) error {
	if existingLock == nil {
		return nil
	}

	stillPresent := make(map[string]bool, len(resolved))
	for _, rs := range resolved {
		stillPresent[rs.SkillDependency.Name] = true
	}

	// A dropped entry no longer exists in the manifest, so its old target list
	// is unknown. Sweep every agent to be certain no merged block survives.
	allTargets := types.SupportedAgentNames()

	for _, entry := range existingLock.Entries {
		if stillPresent[entry.Name] {
			continue
		}
		for _, scope := range []string{types.ScopeProject, types.ScopeGlobal} {
			e.cleanupLinks(entry.Name, scope, allTargets)
		}
	}

	return nil
}

// cleanGlobalOrphans prunes the shared ~/.agents tree. Every project shares it,
// so the valid set is the union recorded in the global ledger, updated with this
// project's current global skills. Pruning against a single project's manifest
// would delete other projects' global installs.
func (e *Engine) cleanGlobalOrphans(resolved []ResolvedSkill) error {
	var globalNames []string
	for _, rs := range resolved {
		if rs.SkillDependency.EffectiveScope() == types.ScopeGlobal {
			globalNames = append(globalNames, rs.SkillDependency.Name)
		}
	}

	ledgerPath := lockfile.GlobalLedgerPath(e.SkmgrHome)

	ledger, err := lockfile.ReadGlobalLedger(ledgerPath)
	if err != nil {
		return fmt.Errorf("reading global ledger: %w", err)
	}

	// Anything in the shared tree that no project claims is something skmgr has
	// no record of installing: a legacy install predating the ledger, or one put
	// there by hand. Adopt it so pruning never deletes it. This has to happen on
	// every run, not just when the ledger is absent — the ledger is created by
	// whichever project installs first, and a later arrival would otherwise be
	// treated as an orphan.
	//
	// Skills this project declares are excluded: they were just installed above,
	// so they would look unclaimed until SetProject records them, and adopting
	// them would make them permanent.
	claimed := ledger.AllNames()
	for _, name := range globalNames {
		claimed[name] = true
	}

	existing, err := e.Installer.ListInstalled(types.ScopeGlobal)
	if err != nil {
		return fmt.Errorf("inspecting existing global installs: %w", err)
	}

	var unclaimed []string
	for _, name := range existing {
		if !claimed[name] {
			unclaimed = append(unclaimed, name)
		}
	}
	ledger.Adopt(unclaimed)

	ledger.SetProject(e.ProjectRoot, globalNames)

	if err := lockfile.WriteGlobalLedger(ledgerPath, ledger); err != nil {
		return fmt.Errorf("updating global ledger: %w", err)
	}

	if err := e.Installer.CleanOrphansByName(ledger.AllNames(), types.ScopeGlobal); err != nil {
		return fmt.Errorf("cleaning global orphans: %w", err)
	}

	return nil
}

// Remove completely uninstalls a skill: it drops every agent projection and
// deletes the canonical copy. Callers that go through Sync do not need this,
// since orphan cleanup covers them; Remove exists for removing a single skill
// without re-resolving the rest of the manifest.
func (e *Engine) Remove(skillName string, scope string, targets []string) error {
	if err := types.ValidateName(skillName); err != nil {
		return fmt.Errorf("invalid skill name: %w", err)
	}

	e.cleanupLinks(skillName, scope, targets)

	if err := e.Installer.RemoveCanonical(skillName, scope); err != nil {
		return fmt.Errorf("removing canonical copy of %q: %w", skillName, err)
	}

	return nil
}

func (e *Engine) cleanupLinks(name string, scope string, targets []string) {
	for _, target := range targets {
		// Try unlinking as both skill and rule to be safe, since we might be converting
		_ = e.Linker.UnlinkSkill(target, name, scope, e.ProjectRoot)
		_ = e.Linker.UnlinkRule(target, name, scope, e.ProjectRoot)
	}
}
