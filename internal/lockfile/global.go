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

package lockfile

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/AAG1999/skmgr/internal/util/atomicfile"
	yaml "gopkg.in/yaml.v3"
)

// GlobalLedgerVersion is the current on-disk format version.
const GlobalLedgerVersion = "1"

// GlobalLedger records which project installed which globally scoped skill.
// Global skills live in a single shared ~/.agents tree, so orphan cleanup can
// only be safe if it knows the union of every project's global installs.
// Without this, installing project A would delete project B's global skills.
type GlobalLedger struct {
	Version string `yaml:"version"`

	// UpdatedAt is the ISO 8601 timestamp of the last write.
	UpdatedAt string `yaml:"updated_at"`

	// Projects maps an absolute project root to the global skill names that
	// project currently declares.
	Projects map[string][]string `yaml:"projects,omitempty"`

	// Adopted holds global skills that already existed when the ledger was
	// first created. skmgr has no record of which project installed them, so
	// they are protected from orphan pruning until the user removes them
	// deliberately with `skmgr clean --global`.
	Adopted []string `yaml:"adopted,omitempty"`
}

// GlobalLedgerPath returns the ledger location inside the skmgr home directory.
func GlobalLedgerPath(skmgrHome string) string {
	return filepath.Join(skmgrHome, "global.lock")
}

// ReadGlobalLedger loads the ledger. A missing file yields an empty ledger
// rather than an error, since the first global install creates it.
func ReadGlobalLedger(path string) (*GlobalLedger, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &GlobalLedger{
				Version:  GlobalLedgerVersion,
				Projects: map[string][]string{},
			}, nil
		}
		return nil, fmt.Errorf("reading global ledger: %w", err)
	}

	var ledger GlobalLedger
	if err := yaml.Unmarshal(data, &ledger); err != nil {
		return nil, fmt.Errorf("parsing global ledger %q: %w", path, err)
	}
	if ledger.Projects == nil {
		ledger.Projects = map[string][]string{}
	}
	if ledger.Version == "" {
		ledger.Version = GlobalLedgerVersion
	}

	return &ledger, nil
}

// WriteGlobalLedger persists the ledger, creating parent directories as needed.
func WriteGlobalLedger(path string, ledger *GlobalLedger) error {
	ledger.Version = GlobalLedgerVersion
	ledger.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating global ledger directory: %w", err)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(ledger); err != nil {
		return fmt.Errorf("encoding global ledger: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing global ledger encoder: %w", err)
	}

	if err := atomicfile.Write(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing global ledger: %w", err)
	}

	return nil
}

// SetProject records the global skill names owned by a project. Passing an
// empty list removes the project from the ledger entirely.
func (g *GlobalLedger) SetProject(projectRoot string, names []string) {
	if g.Projects == nil {
		g.Projects = map[string][]string{}
	}

	key := normalizeProjectKey(projectRoot)
	if len(names) == 0 {
		delete(g.Projects, key)
		return
	}

	sorted := append([]string(nil), names...)
	sort.Strings(sorted)
	g.Projects[key] = sorted
}

// AllNames returns the union of global skill names across every project, plus
// any adopted pre-existing installs. Orphan cleanup must use this set, never a
// single project's view.
func (g *GlobalLedger) AllNames() map[string]bool {
	union := make(map[string]bool)
	for _, names := range g.Projects {
		for _, name := range names {
			union[name] = true
		}
	}
	for _, name := range g.Adopted {
		union[name] = true
	}
	return union
}

// Adopt records pre-existing global installs so the first prune after an
// upgrade does not delete skills that predate the ledger.
func (g *GlobalLedger) Adopt(names []string) {
	seen := make(map[string]bool, len(g.Adopted))
	for _, existing := range g.Adopted {
		seen[existing] = true
	}
	for _, name := range names {
		if !seen[name] {
			g.Adopted = append(g.Adopted, name)
			seen[name] = true
		}
	}
	sort.Strings(g.Adopted)
}

// ReleaseAdopted clears the adopted set. Used by an explicit global clean, at
// which point the user has said they want the shared tree reset.
func (g *GlobalLedger) ReleaseAdopted() {
	g.Adopted = nil
}

// normalizeProjectKey makes ledger keys stable across relative invocations.
func normalizeProjectKey(projectRoot string) string {
	if abs, err := filepath.Abs(projectRoot); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(projectRoot)
}
