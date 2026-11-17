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

	// The default encoder is what every existing lockfile was written with.
	// Use it whenever it round-trips, so ordinary pins stay plain scalars.
	data, err := encodeLockDocument(lock)
	if err != nil {
		return err
	}
	if !lockfileRoundTrips(lock, data) {
		// A block scalar drops a leading newline, so a ref of "\n0" is written
		// back as "0" and the next install checks out a different name. Quote
		// only the strings the default style cannot preserve.
		data, err = encodeLockDocument(faithfulFrom(lock))
		if err != nil {
			return err
		}
	}

	return writeIfUnchanged(path, lock, data)
}

// writeIfUnchanged publishes data only when decoding it yields lock.
//
// The check happens before the file is replaced. A mismatch means the encoder
// changed a field, and publishing that would repin a skill on the next read.
// An existing file keeps its bytes; a path that was absent stays absent.
func writeIfUnchanged(path string, lock *types.Lockfile, data []byte) error {
	if !lockfileRoundTrips(lock, data) {
		return fmt.Errorf("encoding lockfile to YAML: round trip changed a field")
	}

	if err := atomicfile.Write(path, data, 0644); err != nil {
		return fmt.Errorf("writing lockfile: %w", err)
	}

	return nil
}

// encodeLockDocument encodes v with the same indentation Write has always used.
func encodeLockDocument(v any) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(v); err != nil {
		return nil, fmt.Errorf("encoding lockfile to YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("closing YAML encoder: %w", err)
	}

	return buf.Bytes(), nil
}

// lockfileRoundTrips reports whether data decodes to the same version,
// timestamp, and entries as want. Nil and empty entry lists compare equal:
// omitempty writes both as an absent key.
func lockfileRoundTrips(want *types.Lockfile, data []byte) bool {
	var got types.Lockfile
	if err := yaml.Unmarshal(data, &got); err != nil {
		return false
	}
	if got.Version != want.Version || got.GeneratedAt != want.GeneratedAt {
		return false
	}
	if len(got.Entries) != len(want.Entries) {
		return false
	}
	for i := range want.Entries {
		if got.Entries[i] != want.Entries[i] {
			return false
		}
	}
	return true
}

// faithfulLock is the second encoding of a lockfile, used only when the
// default encoder would change a field. Each string keeps the default style
// unless that style does not survive a read.
type faithfulLock struct {
	Version     string          `yaml:"version"`
	GeneratedAt string          `yaml:"generated_at"`
	Entries     []faithfulEntry `yaml:"entries,omitempty"`
}

type faithfulEntry struct {
	Name        faithfulString `yaml:"name"`
	Source      faithfulString `yaml:"source"`
	Path        faithfulString `yaml:"path,omitempty"`
	Ref         faithfulString `yaml:"ref,omitempty"`
	ResolvedRef faithfulString `yaml:"resolved_ref,omitempty"`
	CommitSHA   faithfulString `yaml:"commit_sha,omitempty"`
	ContentHash faithfulString `yaml:"content_hash"`
	ResolvedAt  faithfulString `yaml:"resolved_at"`
}

func faithfulFrom(lock *types.Lockfile) faithfulLock {
	out := faithfulLock{
		Version:     lock.Version,
		GeneratedAt: lock.GeneratedAt,
		Entries:     make([]faithfulEntry, len(lock.Entries)),
	}
	for i, entry := range lock.Entries {
		out.Entries[i] = faithfulEntry{
			Name:        faithfulString(entry.Name),
			Source:      faithfulString(entry.Source),
			Path:        faithfulString(entry.Path),
			Ref:         faithfulString(entry.Ref),
			ResolvedRef: faithfulString(entry.ResolvedRef),
			CommitSHA:   faithfulString(entry.CommitSHA),
			ContentHash: faithfulString(entry.ContentHash),
			ResolvedAt:  faithfulString(entry.ResolvedAt),
		}
	}
	return out
}

// faithfulString lets the encoder choose the style for strings it can already
// round-trip, and double-quotes the rest. Double quotes escape a leading
// newline as \n; a literal block scalar discards it.
type faithfulString string

func (s faithfulString) MarshalYAML() (interface{}, error) {
	if defaultStringRoundTrips(string(s)) {
		return string(s), nil
	}
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!str",
		Style: yaml.DoubleQuotedStyle,
		Value: string(s),
	}, nil
}

// defaultStringRoundTrips reports whether a string field, nested the way a
// lockfile field is nested, unmarshals to the same value the encoder was given.
func defaultStringRoundTrips(s string) bool {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(struct {
		V string `yaml:"v"`
	}{V: s}); err != nil {
		return false
	}
	if err := enc.Close(); err != nil {
		return false
	}

	var got struct {
		V string `yaml:"v"`
	}
	if err := yaml.Unmarshal(buf.Bytes(), &got); err != nil {
		return false
	}
	return got.V == s
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
