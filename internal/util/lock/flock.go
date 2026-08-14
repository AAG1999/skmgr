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

// Package lock provides an advisory file lock for guarding shared state.
//
// The download cache in ~/.skmgr/cache is shared by every skmgr process on the
// machine. Two runs fetching the same repository at once would race on the same
// clone directory, and a CI machine running several jobs is the normal case,
// not an exotic one. Serializing on a lock file is enough: the contended
// operations are short, and the alternative is a corrupt cache.
package lock

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Lock is a held advisory lock. Release it with Unlock.
type Lock struct {
	file *os.File
	path string
}

// Acquire blocks until the lock at path is held, or timeout elapses.
//
// The lock is advisory and only coordinates skmgr with itself, which is all
// that is needed: nothing else writes to the cache.
func Acquire(path string, timeout time.Duration) (*Lock, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("creating lock directory: %w", err)
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("opening lock file %q: %w", path, err)
	}

	deadline := time.Now().Add(timeout)
	backoff := time.Millisecond

	for {
		err := tryLock(file)
		if err == nil {
			return &Lock{file: file, path: path}, nil
		}

		if !isLockBusy(err) {
			_ = file.Close()
			return nil, fmt.Errorf("locking %q: %w", path, err)
		}

		if time.Now().After(deadline) {
			_ = file.Close()
			return nil, fmt.Errorf(
				"timed out after %s waiting for the lock at %q.\n"+
					"Another skmgr process is probably still running; "+
					"if none is, delete that file", timeout, path)
		}

		time.Sleep(backoff)
		if backoff < 100*time.Millisecond {
			backoff *= 2
		}
	}
}

// Unlock releases the lock. It is safe to call on a nil Lock so callers can
// defer it without first checking whether acquisition succeeded.
func (l *Lock) Unlock() error {
	if l == nil || l.file == nil {
		return nil
	}

	err := unlock(l.file)
	closeErr := l.file.Close()
	l.file = nil

	if err != nil {
		return fmt.Errorf("unlocking %q: %w", l.path, err)
	}
	if closeErr != nil {
		return fmt.Errorf("closing lock file %q: %w", l.path, closeErr)
	}
	return nil
}

// WithLock runs fn while holding the lock at path.
func WithLock(path string, timeout time.Duration, fn func() error) error {
	l, err := Acquire(path, timeout)
	if err != nil {
		return err
	}
	defer func() { _ = l.Unlock() }()

	return fn()
}
