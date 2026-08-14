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

// Package lockfile handles reading and writing the skmgr.lock file.
package lockfile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/AAG1999/skmgr/internal/types"
	"github.com/AAG1999/skmgr/internal/util/atomicfile"
	yaml "gopkg.in/yaml.v3"
)

// ErrNotFound reports that no lockfile exists yet, which is normal for a fresh
// project. It is distinct from a lockfile that exists but cannot be read: the
// first means "resolve everything", the second means "stop and tell the user",
// and conflating them silently discards a lockfile the user meant to keep.
var ErrNotFound = errors.New("lockfile not found")

// Read loads the lockfile from the given path.
//
// A missing file returns ErrNotFound. Callers that treat a fresh project as
// normal should check for it with errors.Is rather than ignoring every error.
func Read(path string) (*types.Lockfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("reading lockfile %q: %w", path, err)
	}

	var lock types.Lockfile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		// The path is quoted by hand rather than with %q, which escapes the
		// separators in a Windows path into something the user cannot paste
		// back into a shell to act on this advice.
		return nil, fmt.Errorf(
			`"%s" is not valid YAML: %w`+"\n"+
				"Delete it and re-run 'skmgr install' to regenerate it, "+
				"or restore it from version control", path, err)
	}

	if err := checkVersion(path, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// checkVersion refuses a lockfile this binary does not understand. Installing
// from one written by a newer skmgr would silently ignore fields that exist to
// constrain the install, which defeats the point of having a lockfile.
//
// Older versions are readable, because every version has only added optional
// fields. Such a lockfile is upgraded in place and stamped on the next write,
// so a team member on an older skmgr is not left with an unreadable file the
// moment someone else upgrades.
func checkVersion(path string, lock *types.Lockfile) error {
	if lock.Version == "" {
		// Pre-versioning lockfiles are readable; the next write stamps them.
		lock.Version = types.LockfileVersion
		return nil
	}

	for _, supported := range types.SupportedLockfileVersions {
		if lock.Version == supported {
			lock.Version = types.LockfileVersion
			return nil
		}
	}

	return fmt.Errorf(
		"%q has lockfile version %q, but this skmgr understands %s.\n"+
			"Upgrade skmgr, or delete the lockfile and re-run 'skmgr install' to regenerate it",
		path, lock.Version, strings.Join(types.SupportedLockfileVersions, " and "))
}

// Write saves the lockfile to the given path.
// It automatically updates the GeneratedAt timestamp and sorts entries by name.
func Write(path string, lock *types.Lockfile) error {
	lock.GeneratedAt = time.Now().UTC().Format(time.RFC3339)

	// Sort entries by name for determinism.
	sort.Slice(lock.Entries, func(i, j int) bool {
		return lock.Entries[i].Name < lock.Entries[j].Name
	})

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(lock); err != nil {
		return fmt.Errorf("encoding lockfile to YAML: %w", err)
	}

	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing YAML encoder: %w", err)
	}

	if err := atomicfile.Write(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing lockfile: %w", err)
	}

	return nil
}

// HashDirectory computes a deterministic SHA-256 hash of a directory's contents.
// It hashes file paths (relative to root) and their contents, ignoring directories.
// Symlinks contribute their target string rather than the content they point at,
// so the hash stays stable and never follows a link off the tree.
// The result is prefixed with "sha256:".
func HashDirectory(root string) (string, error) {
	h := sha256.New()

	type entry struct {
		rel       string
		isSymlink bool
	}

	var files []entry
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Skip directories, we only hash file contents and their relative paths
		if !d.IsDir() {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			// Use forward slashes for determinism across OSes
			files = append(files, entry{
				rel:       filepath.ToSlash(rel),
				isSymlink: d.Type()&fs.ModeSymlink != 0,
			})
		}
		return nil
	})
	if err != nil {
		return "", fmt.Errorf("walking directory for hash: %w", err)
	}

	// Sort files to ensure deterministic hashing
	sort.Slice(files, func(i, j int) bool { return files[i].rel < files[j].rel })

	for _, file := range files {
		// Include file path in hash
		h.Write([]byte(file.rel))
		h.Write([]byte{0}) // separator

		fullPath := filepath.Join(root, filepath.FromSlash(file.rel))

		if file.isSymlink {
			target, err := os.Readlink(fullPath)
			if err != nil {
				return "", fmt.Errorf("reading symlink for hash %s: %w", file.rel, err)
			}
			h.Write([]byte("symlink:"))
			h.Write([]byte(filepath.ToSlash(target)))
			continue
		}

		// Include file contents
		f, err := os.Open(fullPath)
		if err != nil {
			return "", fmt.Errorf("opening file for hash %s: %w", file.rel, err)
		}

		if _, err := io.Copy(h, f); err != nil {
			_ = f.Close()
			return "", fmt.Errorf("hashing file contents %s: %w", file.rel, err)
		}
		_ = f.Close()
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}
