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

package lock

import (
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAcquire_AndRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if err := l.Unlock(); err != nil {
		t.Fatalf("Unlock: %v", err)
	}

	// Releasing must actually release, or the next run of skmgr hangs.
	second, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("re-acquiring after release: %v", err)
	}
	if err := second.Unlock(); err != nil {
		t.Fatal(err)
	}
}

// Unlock on a nil Lock lets callers defer it without checking the error first.
func TestUnlock_NilIsSafe(t *testing.T) {
	var l *Lock
	if err := l.Unlock(); err != nil {
		t.Errorf("Unlock on nil should be a no-op, got: %v", err)
	}
}

func TestAcquire_TimesOutWhenHeld(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	held, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Unlock() }()

	start := time.Now()
	_, err = Acquire(path, 100*time.Millisecond)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected a timeout while the lock is held")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Errorf("error should say it timed out, got: %v", err)
	}
	if elapsed < 100*time.Millisecond {
		t.Errorf("returned after %s, should have waited the full timeout", elapsed)
	}
}

// The reason the package exists: concurrent skmgr runs must not interleave
// inside the guarded section.
func TestWithLock_SerializesContendingGoroutines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	var inside atomic.Int32
	var maxSeen atomic.Int32
	var overlaps atomic.Int32

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			err := WithLock(path, 10*time.Second, func() error {
				n := inside.Add(1)
				if n > 1 {
					overlaps.Add(1)
				}
				for {
					current := maxSeen.Load()
					if n <= current || maxSeen.CompareAndSwap(current, n) {
						break
					}
				}
				time.Sleep(5 * time.Millisecond)
				inside.Add(-1)
				return nil
			})
			if err != nil {
				t.Errorf("WithLock: %v", err)
			}
		}()
	}
	wg.Wait()

	if overlaps.Load() != 0 {
		t.Errorf("%d goroutines entered the critical section concurrently", overlaps.Load())
	}
	if maxSeen.Load() != 1 {
		t.Errorf("max concurrent holders = %d, want 1", maxSeen.Load())
	}
}

// The lock must be released even when the guarded work fails.
func TestWithLock_ReleasesOnError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")

	sentinel := errPlaceholder{}
	err := WithLock(path, time.Second, func() error { return sentinel })
	if err != sentinel {
		t.Fatalf("WithLock should return the callback's error, got: %v", err)
	}

	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("lock was not released after a failure: %v", err)
	}
	_ = l.Unlock()
}

type errPlaceholder struct{}

func (errPlaceholder) Error() string { return "work failed" }

// Parent directories are created so callers can lock inside a cache dir that
// does not exist yet.
func TestAcquire_CreatesParentDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deeper", "test.lock")

	l, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	_ = l.Unlock()
}
