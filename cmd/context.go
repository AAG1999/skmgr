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

package cmd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/types"
)

// LockfileName is the fixed lockfile name beside the manifest.
const LockfileName = "skmgr.lock"

// projectContext bundles the paths and parsed data every command needs, so
// manifest discovery and lockfile loading behave identically everywhere.
type projectContext struct {
	Root         string
	ManifestPath string
	LockPath     string
	Manifest     *types.Manifest
	Lock         *types.Lockfile
}

// loadProject finds the manifest in the working directory, parses it, and
// loads the lockfile if one is present.
//
// Manifest discovery goes through manifest.FindManifestPath, so `skmgr.yaml`
// works exactly like `skmgr.yml` in every command rather than only in
// discovery.
func loadProject() (*projectContext, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("determining working directory: %w", err)
	}

	manifestPath, err := manifest.FindManifestPath(cwd)
	if err != nil {
		return nil, err
	}

	m, err := manifest.Parse(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filepath.Base(manifestPath), err)
	}

	lockPath := filepath.Join(cwd, LockfileName)
	lock, err := lockfile.Read(lockPath)
	if err != nil && !errors.Is(err, lockfile.ErrNotFound) {
		// A lockfile that exists but cannot be read is corruption, not absence.
		// Overwriting it would silently discard the pinned state, so stop and
		// let the user decide.
		return nil, err
	}

	return &projectContext{
		Root:         cwd,
		ManifestPath: manifestPath,
		LockPath:     lockPath,
		Manifest:     m,
		Lock:         lock,
	}, nil
}
