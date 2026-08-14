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

// Package atomicfile writes files so that readers never observe a partial one.
//
// skmgr rewrites files that the user cannot easily reconstruct: the manifest,
// the lockfile, and shared agent rule files like CLAUDE.md. A plain
// os.WriteFile truncates first and writes second, so an interrupted run leaves
// a truncated file and the original content is gone. Writing to a temporary
// file in the same directory and renaming over the target makes the swap
// atomic: a reader sees either the old file or the new one.
package atomicfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

// Write writes data to path atomically, creating parent directories as needed.
//
// The temporary file is created in the target's own directory because rename is
// only atomic within a filesystem, and /tmp is frequently a different one.
func Write(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating directory %q: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating temporary file in %q: %w", dir, err)
	}
	tmpName := tmp.Name()

	// Any failure from here on must not leave the temporary file behind.
	cleanup := func() {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
	}

	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("writing temporary file for %q: %w", path, err)
	}

	// Without the sync, a crash shortly after the rename can leave the new
	// name pointing at a file whose contents were never flushed.
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("syncing temporary file for %q: %w", path, err)
	}

	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("closing temporary file for %q: %w", path, err)
	}

	// CreateTemp always uses 0600; apply the caller's mode before publishing.
	if err := os.Chmod(tmpName, perm); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("setting permissions on temporary file for %q: %w", path, err)
	}

	if err := replace(tmpName, path); err != nil {
		_ = os.Remove(tmpName)
		return fmt.Errorf("replacing %q: %w", path, err)
	}

	// Fsync the directory so the rename itself survives a crash. Not all
	// platforms allow opening a directory for this, so a failure here is not
	// treated as fatal: the data is already durable, only the rename might not
	// be.
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}

	return nil
}

// replaceRetries and replaceBackoff bound how long a replace waits for a
// competing reader on Windows to let go.
const (
	replaceRetries = 20
	replaceBackoff = 10 * time.Millisecond
)

// replace renames tmp over path.
//
// On Unix this is a single rename and always succeeds. Windows refuses to
// replace a file that another process has open, and the files skmgr rewrites —
// the lockfile, CLAUDE.md — are exactly the ones an editor or a running agent
// is likely to be reading. Those handles are held for microseconds, so a short
// retry turns a spurious "Access is denied" into a successful write instead of
// a failed install.
func replace(tmp string, path string) error {
	if runtime.GOOS != "windows" {
		return os.Rename(tmp, path)
	}

	var err error
	for attempt := 0; attempt < replaceRetries; attempt++ {
		if err = os.Rename(tmp, path); err == nil {
			return nil
		}
		if !errors.Is(err, fs.ErrPermission) {
			return err
		}
		time.Sleep(replaceBackoff)
	}
	return err
}
