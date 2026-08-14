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

package provider

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/AAG1999/skmgr/internal/log"
)

// legacyCacheSweeps remembers which cache directories have been swept, so a
// single run does not re-scan the cache once per skill.
var legacyCacheSweeps sync.Map

// dropLegacyCache removes cache entries written by versions that predate the
// SHA-addressed layout.
//
// The old layout put one working clone per source directly under the cache
// root, keyed by a hex digest of the source URL, and checked the requested ref
// out inside it. Those directories are unreadable to the current code and can
// be large, so they are deleted rather than migrated: everything in them is
// re-fetchable, and the only cost of dropping them is one slower run.
//
// Failures are logged and ignored. A cache that cannot be tidied is a nuisance,
// never a reason to fail an install.
func dropLegacyCache(cacheDir string) {
	if _, swept := legacyCacheSweeps.LoadOrStore(cacheDir, true); swept {
		return
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() || !isLegacyCacheDirName(entry.Name()) {
			continue
		}

		path := filepath.Join(cacheDir, entry.Name())
		log.Debugf("removing pre-SHA-addressed cache entry %s", path)

		if err := os.RemoveAll(path); err != nil {
			log.Warnf("could not remove the outdated cache entry %s: %v", path, err)
		}
	}
}

// isLegacyCacheDirName reports whether a name looks like an old per-source
// cache directory: the hex digest of a source URL. The current layout keeps
// everything under "git", so nothing this matches is still in use.
func isLegacyCacheDirName(name string) bool {
	if len(name) != 64 {
		return false
	}

	for _, r := range name {
		isHex := (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')
		if !isHex {
			return false
		}
	}

	return true
}
