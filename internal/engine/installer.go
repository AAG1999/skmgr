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
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/types"
)

// HashMismatchError reports that fetched content does not match the hash the
// lockfile recorded. It carries its own full explanation so callers can surface
// it without wrapping it in another layer of context the user does not need.
type HashMismatchError struct {
	Name     string
	Expected string
	Actual   string
}

func (e *HashMismatchError) Error() string {
	return fmt.Sprintf(
		"%q does not match the lockfile.\n"+
			"  lockfile: %s\n"+
			"  fetched:  %s\n"+
			"The source changed under a ref that was supposed to be pinned, or the "+
			"cache is corrupt. Run 'skmgr clean --cache' and retry, or run "+
			"'skmgr install' without --frozen to accept the new content",
		e.Name, e.Expected, e.Actual)
}

// Installer handles moving fetched skills into the .agents canonical directories.
type Installer struct {
	projectRoot string
}

// NewInstaller creates a new Installer.
func NewInstaller(projectRoot string) *Installer {
	return &Installer{
		projectRoot: projectRoot,
	}
}

// Install copies the skill from the cache source to its canonical directory and
// computes the content hash.
func (i *Installer) Install(resolved ResolvedSkill) (string, error) {
	return i.install(resolved, "")
}

// InstallExpecting is Install with an integrity check. If the staged content
// does not hash to expectedHash, nothing is swapped in and the existing install
// is left alone. A frozen install uses this so a mismatch is refused rather
// than discovered after the damage is done.
//
// An empty expectedHash skips the check.
func (i *Installer) InstallExpecting(resolved ResolvedSkill, expectedHash string) (string, error) {
	return i.install(resolved, expectedHash)
}

func (i *Installer) install(resolved ResolvedSkill, expectedHash string) (string, error) {
	// Names reach here from the manifest and from wildcard auto-discovery, so
	// re-check before using one as a path component.
	if err := types.ValidateName(resolved.SkillDependency.Name); err != nil {
		return "", fmt.Errorf("invalid skill name: %w", err)
	}

	scope := resolved.SkillDependency.EffectiveScope()

	var destBase, siblingBase string
	if resolved.SkillDependency.EffectiveType() == types.TypeSkill {
		destBase = types.CanonicalSkillsDir(scope, i.projectRoot)
		siblingBase = types.CanonicalRulesDir(scope, i.projectRoot)
	} else {
		destBase = types.CanonicalRulesDir(scope, i.projectRoot)
		siblingBase = types.CanonicalSkillsDir(scope, i.projectRoot)
	}

	// A dependency that flipped type leaves a copy under the other tree. Orphan
	// cleanup keys on name alone, so it would consider that copy still valid and
	// the agent would keep seeing the stale version.
	sibling := filepath.Join(siblingBase, resolved.SkillDependency.Name)
	if err := os.RemoveAll(sibling); err != nil {
		return "", fmt.Errorf("removing stale %q: %w", sibling, err)
	}

	destDir := filepath.Join(destBase, resolved.SkillDependency.Name)

	if err := os.MkdirAll(destBase, 0755); err != nil {
		return "", fmt.Errorf("creating canonical directory %q: %w", destBase, err)
	}

	// Copying straight over the destination means a failure part way through
	// leaves a half-written skill that looks installed. Build the new copy in a
	// staging directory first and swap it in only once it is complete.
	staging, err := i.stagingDir(scope)
	if err != nil {
		return "", err
	}

	tmpDir, err := os.MkdirTemp(staging, resolved.SkillDependency.Name+"-*")
	if err != nil {
		return "", fmt.Errorf("creating staging directory for %q: %w", resolved.SkillDependency.Name, err)
	}
	defer func() {
		_ = os.RemoveAll(tmpDir)
		// Remove succeeds only when the staging area is empty, which is what we
		// want: leave nothing behind, but never disturb a concurrent install.
		_ = os.Remove(staging)
	}()

	// MkdirTemp creates the directory, but copyDir wants to create it itself.
	staged := filepath.Join(tmpDir, "payload")

	if err := copyDir(resolved.SourceDir, staged); err != nil {
		return "", fmt.Errorf("copying skill from %q: %w", resolved.SourceDir, err)
	}

	// Hash the staged copy: it is what is about to become the install, and
	// hashing before the swap keeps the window where destDir is absent short.
	hash, err := lockfile.HashDirectory(staged)
	if err != nil {
		return "", fmt.Errorf("computing content hash for %q: %w", resolved.SkillDependency.Name, err)
	}

	// Check before the swap, so a rejected install leaves the previous one in
	// place instead of being detected after it has already been replaced.
	if expectedHash != "" && hash != expectedHash {
		return "", &HashMismatchError{
			Name:     resolved.SkillDependency.Name,
			Expected: expectedHash,
			Actual:   hash,
		}
	}

	// Rename cannot replace a non-empty directory, so the old copy goes first.
	// This is the one unavoidable gap; keeping it to two adjacent syscalls is
	// the best a plain filesystem allows.
	if err := os.RemoveAll(destDir); err != nil {
		return "", fmt.Errorf("removing existing directory %q: %w", destDir, err)
	}

	if err := os.Rename(staged, destDir); err != nil {
		return "", fmt.Errorf("installing %q: %w", destDir, err)
	}

	return hash, nil
}

// stagingDir returns the scope's staging area, creating it if needed. It lives
// inside the canonical tree so the final rename stays on one filesystem.
func (i *Installer) stagingDir(scope string) (string, error) {
	dir := filepath.Join(types.CanonicalDir(scope, i.projectRoot), ".staging")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("creating staging directory %q: %w", dir, err)
	}
	return dir, nil
}

// CleanOrphans removes any directories in the canonical locations that are not present in the resolved skills.
func (i *Installer) CleanOrphans(resolved []ResolvedSkill, scope string) error {
	validNames := make(map[string]bool)
	for _, rs := range resolved {
		if rs.SkillDependency.EffectiveScope() == scope {
			validNames[rs.SkillDependency.Name] = true
		}
	}

	return i.CleanOrphansByName(validNames, scope)
}

// RemoveCanonical deletes a skill's canonical directory. Both the skills and
// rules trees are checked, since a dependency's type may have changed since it
// was installed.
func (i *Installer) RemoveCanonical(name string, scope string) error {
	if err := types.ValidateName(name); err != nil {
		return err
	}

	for _, baseDir := range []string{
		types.CanonicalSkillsDir(scope, i.projectRoot),
		types.CanonicalRulesDir(scope, i.projectRoot),
	} {
		if err := os.RemoveAll(filepath.Join(baseDir, name)); err != nil {
			return err
		}
	}

	return nil
}

// ListInstalled returns the names of every skill and rule currently present in
// the canonical directories for a scope.
func (i *Installer) ListInstalled(scope string) ([]string, error) {
	var names []string

	for _, baseDir := range []string{
		types.CanonicalSkillsDir(scope, i.projectRoot),
		types.CanonicalRulesDir(scope, i.projectRoot),
	} {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				names = append(names, entry.Name())
			}
		}
	}

	return names, nil
}

// CleanOrphansByName prunes canonical directories against an explicit set of
// valid names. The global tree is shared between projects, so its valid set has
// to be assembled from the ledger rather than one project's resolved skills.
func (i *Installer) CleanOrphansByName(validNames map[string]bool, scope string) error {
	cleanDir := func(baseDir string) error {
		entries, err := os.ReadDir(baseDir)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			if !validNames[entry.Name()] {
				// Orphan found, remove it
				orphanPath := filepath.Join(baseDir, entry.Name())
				if err := os.RemoveAll(orphanPath); err != nil {
					return fmt.Errorf("removing orphan %q: %w", orphanPath, err)
				}
			}
		}
		return nil
	}

	if err := cleanDir(types.CanonicalSkillsDir(scope, i.projectRoot)); err != nil {
		return err
	}
	if err := cleanDir(types.CanonicalRulesDir(scope, i.projectRoot)); err != nil {
		return err
	}

	return nil
}

// copyDir recursively copies a directory tree. Symlinks are recreated rather
// than dereferenced, but only when they resolve inside the source tree; a link
// escaping the skill would otherwise smuggle host files into the install.
func copyDir(src string, dst string) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("resolving source directory: %w", err)
	}

	return filepath.Walk(absSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Don't copy .git directories from the cache
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}

		relPath, err := filepath.Rel(absSrc, path)
		if err != nil {
			return err
		}

		targetPath := filepath.Join(dst, relPath)

		if info.Mode()&os.ModeSymlink != 0 {
			return copySymlink(absSrc, path, targetPath)
		}

		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}

		// It's a file, copy it
		return copyFile(path, targetPath, info.Mode())
	})
}

// copySymlink recreates a symlink at targetPath. The link's resolved
// destination must stay within srcRoot so a skill cannot reach outside itself.
func copySymlink(srcRoot string, path string, targetPath string) error {
	linkDest, err := os.Readlink(path)
	if err != nil {
		return fmt.Errorf("reading symlink %q: %w", path, err)
	}

	resolved := linkDest
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(path), resolved)
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return fmt.Errorf("resolving symlink target for %q: %w", path, err)
	}

	if resolved != srcRoot && !strings.HasPrefix(resolved, srcRoot+string(filepath.Separator)) {
		return fmt.Errorf("symlink %q points outside the skill directory (to %q)", path, linkDest)
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("creating parent directory for symlink %q: %w", targetPath, err)
	}
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("clearing existing path %q: %w", targetPath, err)
	}
	if err := os.Symlink(linkDest, targetPath); err != nil {
		return fmt.Errorf("creating symlink %q: %w", targetPath, err)
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	defer func() { _ = out.Close() }()

	if _, err = io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}
