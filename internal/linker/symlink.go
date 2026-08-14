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

// Package linker manages symlinking and merging skill/rule contents.
package linker

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/AAG1999/skmgr/internal/log"
)

// symlink and junction are indirected so the fallback chain can be exercised
// in tests on any platform. Nothing outside of tests should reassign them.
var (
	symlink  = os.Symlink
	junction = createJunction
)

// createSymlink creates a symlink at linkPath pointing to target.
//
// The link is written relative to its own directory when both sides live on the
// same volume, so that moving or renaming the project — or mounting it at a
// different path inside a container — does not break every link.
//
// If a symlink already exists at linkPath it is replaced unless it already
// records exactly the same target; a real file or directory there is left alone
// and reported, since it is not ours to delete.
func createSymlink(target string, linkPath string) error {
	if err := os.MkdirAll(filepath.Dir(linkPath), 0755); err != nil {
		return fmt.Errorf("creating parent directories for symlink: %w", err)
	}

	want := relativeTarget(target, linkPath)

	info, err := os.Lstat(linkPath)
	switch {
	case err == nil && info.Mode()&os.ModeSymlink != 0:
		existing, err := os.Readlink(linkPath)
		if err != nil {
			return fmt.Errorf("reading existing symlink: %w", err)
		}
		if existing == want {
			return nil
		}
		// Anything else, including a link left absolute by an older skmgr, is
		// rewritten so repeated syncs converge on the relative form.
		if err := os.Remove(linkPath); err != nil {
			return fmt.Errorf("removing incorrect symlink: %w", err)
		}

	case err == nil && isManagedFallback(linkPath):
		// A previous run could not create a link and left a copy behind.
		// Refreshing it is the only way to pick up new content.
		if err := os.RemoveAll(linkPath); err != nil {
			return fmt.Errorf("removing previous copy: %w", err)
		}

	case err == nil:
		return fmt.Errorf("path %q already exists and is not a symlink", linkPath)

	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("statting symlink path: %w", err)
	}

	return linkWithFallback(target, want, linkPath)
}

// relativeTarget expresses target relative to the link's own directory, and
// falls back to the absolute path when no relative route exists — across
// volumes on Windows, for instance, where filepath.Rel refuses.
func relativeTarget(target string, linkPath string) string {
	absTarget, err := filepath.Abs(target)
	if err != nil {
		return target
	}

	absLink, err := filepath.Abs(linkPath)
	if err != nil {
		return absTarget
	}

	rel, err := filepath.Rel(filepath.Dir(absLink), absTarget)
	if err != nil {
		return absTarget
	}
	return rel
}

// linkWithFallback creates the link, degrading only as far as it has to.
//
// Symlinks need Developer Mode or elevation on Windows, so a permission failure
// there is a configuration problem rather than a broken install; a directory
// junction gets the same result without the privilege. When even that is
// unavailable the content is copied, which works but will not track later
// changes, so it is worth a warning.
func linkWithFallback(absTarget string, relTarget string, linkPath string) error {
	symlinkErr := symlink(relTarget, linkPath)
	if symlinkErr == nil {
		return nil
	}

	if !isPermissionDenied(symlinkErr) {
		return fmt.Errorf("creating symlink: %w", symlinkErr)
	}

	resolved, err := filepath.Abs(absTarget)
	if err != nil {
		resolved = absTarget
	}

	if info, statErr := os.Stat(resolved); statErr == nil && info.IsDir() {
		if junctionErr := junction(resolved, linkPath); junctionErr == nil {
			log.Debugf("used a directory junction for %s (symlinks are not permitted)", linkPath)
			return nil
		}
	}

	if copyErr := copyAsFallback(resolved, linkPath); copyErr != nil {
		return fmt.Errorf("creating symlink: %w (copying instead also failed: %v)", symlinkErr, copyErr)
	}

	log.Warnf("copied %s instead of linking it: %v", linkPath, symlinkErr)
	log.Warnf("the copy will not follow later updates; enable Developer Mode on Windows, or run skmgr sync after each change")
	return nil
}

// isPermissionDenied reports whether an error is the operating system refusing
// the operation, which is the case worth degrading for. A missing parent or a
// name collision is a real bug and should surface as one.
func isPermissionDenied(err error) bool {
	return errors.Is(err, os.ErrPermission) || errors.Is(err, fs.ErrPermission)
}

// fallbackMarker names the file dropped beside copied content so a later run
// can tell its own copy apart from a directory the user created.
const fallbackMarker = ".skmgr-copy"

// copyAsFallback duplicates the target where the link should have been.
func copyAsFallback(target string, linkPath string) error {
	info, err := os.Stat(target)
	if err != nil {
		return err
	}

	if !info.IsDir() {
		if err := copyFileContents(target, linkPath, info.Mode()); err != nil {
			return err
		}
		return nil
	}

	if err := copyTree(target, linkPath); err != nil {
		return err
	}

	return os.WriteFile(filepath.Join(linkPath, fallbackMarker), []byte(
		"Created by skmgr because a symlink could not be made.\nThis directory is regenerated by skmgr sync.\n",
	), 0644)
}

// isManagedFallback reports whether a directory is a copy skmgr made earlier.
func isManagedFallback(path string) bool {
	_, err := os.Stat(filepath.Join(path, fallbackMarker))
	return err == nil
}

// copyTree copies a directory recursively, skipping symlinks: the fallback
// exists precisely because links are unavailable on this filesystem.
func copyTree(src string, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		switch {
		case info.Mode()&os.ModeSymlink != 0:
			return nil
		case info.IsDir():
			return os.MkdirAll(target, 0755)
		default:
			return copyFileContents(path, target, info.Mode())
		}
	})
}

// copyFileContents writes one file to another path.
func copyFileContents(src string, dst string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

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

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// isSymlink returns true if the path exists and is a symlink.
func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// linkTarget returns the absolute path a symlink points at.
//
// A relative link is resolved against the directory holding the link, which is
// what the operating system does. Resolving it against the working directory
// instead would make an intact link look broken from anywhere but the project
// root, and skmgr deletes links it believes are broken.
func linkTarget(linkPath string) (string, error) {
	dest, err := os.Readlink(linkPath)
	if err != nil {
		return "", err
	}

	if filepath.IsAbs(dest) {
		return dest, nil
	}
	return filepath.Join(filepath.Dir(linkPath), dest), nil
}

// linkTargetExists reports whether a symlink still resolves to something.
func linkTargetExists(linkPath string) bool {
	target, err := linkTarget(linkPath)
	if err != nil {
		return false
	}

	_, err = os.Stat(target)
	return err == nil
}

// removeSymlink safely removes a symlink. It returns an error if the path
// exists but is not a symlink (to prevent accidental deletion of real files).
func removeSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil // Already gone
		}
		return fmt.Errorf("statting path to remove: %w", err)
	}

	if info.Mode()&os.ModeSymlink == 0 {
		if isManagedFallback(path) {
			if err := os.RemoveAll(path); err != nil {
				return fmt.Errorf("removing copied skill: %w", err)
			}
			return nil
		}
		return fmt.Errorf("refusing to remove %q: it is not a symlink", path)
	}

	if err := os.Remove(path); err != nil {
		return fmt.Errorf("removing symlink: %w", err)
	}

	return nil
}
