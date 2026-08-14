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
	"os"
	"path/filepath"

	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/types"
)

// VerifyStatus is the outcome of checking one lock entry against disk.
type VerifyStatus string

const (
	// VerifyOK means the installed content hashes to what the lockfile records.
	VerifyOK VerifyStatus = "ok"

	// VerifyMissing means the lockfile has an entry with nothing installed.
	VerifyMissing VerifyStatus = "missing"

	// VerifyModified means the content on disk differs from the lockfile.
	VerifyModified VerifyStatus = "modified"

	// VerifyUnverifiable means the lock entry predates content hashing, so
	// there is nothing to compare against. Reported rather than assumed good.
	VerifyUnverifiable VerifyStatus = "unverifiable"
)

// VerifyResult describes one checked entry.
type VerifyResult struct {
	Name     string
	Status   VerifyStatus
	Location string
	Expected string
	Actual   string
}

// OK reports whether this result should count as a success.
func (r VerifyResult) OK() bool {
	return r.Status == VerifyOK || r.Status == VerifyUnverifiable
}

// Verify checks every lockfile entry against what is installed on disk. It
// never touches the network, so it is safe in CI and on a plane.
//
// The lockfile does not record scope or type, so each name is looked for in all
// four canonical locations. That also makes verification correct for wildcard
// entries, whose names only exist in the lockfile.
func (e *Engine) Verify(lock *types.Lockfile) []VerifyResult {
	if lock == nil {
		return nil
	}

	results := make([]VerifyResult, 0, len(lock.Entries))

	for _, entry := range lock.Entries {
		results = append(results, e.verifyEntry(entry))
	}

	return results
}

func (e *Engine) verifyEntry(entry types.LockEntry) VerifyResult {
	result := VerifyResult{
		Name:     entry.Name,
		Expected: entry.ContentHash,
	}

	dir, found := e.locateInstalled(entry.Name)
	if !found {
		result.Status = VerifyMissing
		return result
	}
	result.Location = dir

	if entry.ContentHash == "" {
		result.Status = VerifyUnverifiable
		return result
	}

	actual, err := lockfile.HashDirectory(dir)
	if err != nil {
		// An unreadable install is as unusable as a missing one.
		result.Status = VerifyMissing
		return result
	}
	result.Actual = actual

	if actual != entry.ContentHash {
		result.Status = VerifyModified
		return result
	}

	result.Status = VerifyOK
	return result
}

// locateInstalled finds the canonical directory holding a name, searching both
// scopes and both types.
func (e *Engine) locateInstalled(name string) (string, bool) {
	if err := types.ValidateName(name); err != nil {
		return "", false
	}

	for _, scope := range []string{types.ScopeProject, types.ScopeGlobal} {
		for _, base := range []string{
			types.CanonicalSkillsDir(scope, e.ProjectRoot),
			types.CanonicalRulesDir(scope, e.ProjectRoot),
		} {
			candidate := filepath.Join(base, name)
			if info, err := os.Stat(candidate); err == nil && info.IsDir() {
				return candidate, true
			}
		}
	}

	return "", false
}
