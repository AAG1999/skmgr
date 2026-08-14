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

// Package manifest handles reading, writing, and validating skmgr.yml files.
package manifest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
	yaml "gopkg.in/yaml.v3"
)

// ManifestFilenames are the accepted manifest filenames, in priority order.
var ManifestFilenames = []string{"skmgr.yml", "skmgr.yaml"}

// FindManifestPath searches for a manifest file in the given directory.
// Returns the full path to the manifest, or an error if none is found.
func FindManifestPath(dir string) (string, error) {
	for _, name := range ManifestFilenames {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("no manifest found in %s (expected %s)", dir, strings.Join(ManifestFilenames, " or "))
}

// Parse reads and validates a manifest from the given file path.
func Parse(path string) (*types.Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading manifest: %w", err)
	}
	return ParseBytes(data)
}

// ParseBytes parses and validates a manifest from raw YAML bytes.
func ParseBytes(data []byte) (*types.Manifest, error) {
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, fmt.Errorf("manifest is empty")
	}

	// Unknown keys are rejected rather than ignored. A misspelled `pth:` would
	// otherwise parse cleanly and be silently dropped, leaving the user to work
	// out why their skill installed from the wrong directory.
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)

	var m types.Manifest
	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("parsing manifest YAML: %w", err)
	}

	// Line numbers are read separately: the decoded struct has no memory of
	// where each field was written, and a validation error that cannot point at
	// a line leaves the user counting list entries by hand.
	loc := newLocations(data)

	// Derive names before validating so every consumer downstream — engine,
	// resolver, linker, lockfile, commands — sees a populated Name and needs no
	// fallback logic of its own.
	if err := deriveNames(&m, loc); err != nil {
		return nil, err
	}

	if err := validate(&m, loc); err != nil {
		return nil, err
	}

	return &m, nil
}

// deriveNames fills in the Name of every non-wildcard skill that omitted one.
// A user-supplied name always wins: it is treated as a deliberate override for
// aliasing or for disambiguating two skills that would otherwise collide.
func deriveNames(m *types.Manifest, loc *locations) error {
	for i := range m.Skills {
		skill := &m.Skills[i]

		if strings.TrimSpace(skill.Name) != "" {
			continue
		}

		// Wildcard entries expand into many skills; the resolver names each
		// match from its own directory.
		if IsWildcard(skill.Path) {
			continue
		}

		derived, err := DeriveName(skill.Source, skill.Path)
		if err != nil {
			return at(loc.skillLine(i), "skills[%d]: %v", i, err)
		}

		if err := types.ValidateName(derived); err != nil {
			return at(loc.skillLine(i), "skills[%d]: derived name is unusable: %v", i, err)
		}

		skill.Name = derived
		skill.NameDerived = true
	}

	return nil
}

// validate checks the manifest for required fields and constraint violations.
func validate(m *types.Manifest, loc *locations) error {
	// Name is required.
	if strings.TrimSpace(m.Name) == "" {
		return at(loc.topLine("name"), "'name' is required")
	}

	if err := validateTargets("targets", m.Targets, loc.topLine("targets")); err != nil {
		return err
	}

	// Validate each skill entry.
	//
	// Names are compared case-insensitively because macOS and Windows
	// filesystems are: "Greeter" and "greeter" are two manifest entries but one
	// directory, and whichever installs second silently wins.
	seen := make(map[string]string)
	for i, skill := range m.Skills {
		// Source is required. Checked first because name derivation depends on
		// it, so a missing source produces a clearer error here.
		if strings.TrimSpace(skill.Source) == "" {
			return at(loc.skillLine(i), "skills[%d] is missing 'source'", i)
		}

		// A source or ref beginning with a dash is handed to git as a bare
		// argument, and git decides what an argument means from its first
		// character. `--upload-pack=...` is not a URL git fails to clone; it is
		// an option that runs a command. The providers refuse these too, but
		// catching it here names the line instead of failing mid-fetch.
		if err := types.ValidateGitArgument("source", strings.TrimSpace(skill.Source)); err != nil {
			return at(loc.skillKeyLine(i, "source"), "skills[%d]: %v", i, err)
		}
		if err := types.ValidateGitArgument("ref", strings.TrimSpace(skill.Ref)); err != nil {
			return at(loc.skillKeyLine(i, "ref"), "skills[%d]: %v", i, err)
		}

		// By this point deriveNames has run, so a name is only absent for
		// wildcard entries, whose names are generated per match at resolve time.
		if strings.TrimSpace(skill.Name) == "" && !IsWildcard(skill.Path) {
			return at(loc.skillLine(i), "skills[%d] has no usable 'name'", i)
		}

		// Name must be unique and safe to use as a directory component.
		// Duplicate detection runs on the effective (possibly derived) name.
		if skill.Name != "" {
			if err := types.ValidateName(skill.Name); err != nil {
				return at(loc.skillKeyLine(i, "name"), "skills[%d]: %v", i, err)
			}

			key := strings.ToLower(skill.Name)
			switch previous, clash := seen[key]; {
			case clash && previous == skill.Name:
				return at(loc.skillLine(i),
					"duplicate skill name %q; set an explicit 'name' on one of them to disambiguate",
					skill.Name)
			case clash:
				return at(loc.skillLine(i),
					"skill names %q and %q differ only in case, "+
						"which collide on macOS and Windows; rename one of them",
					previous, skill.Name)
			}
			seen[key] = skill.Name
		}

		// Type must be valid if set.
		if skill.Type != "" && !slices.Contains(types.ValidTypes(), skill.Type) {
			return at(loc.skillKeyLine(i, "type"), "skill %q has invalid type %q (valid: %s)",
				skill.Name, skill.Type, strings.Join(types.ValidTypes(), ", "))
		}

		// Scope must be valid if set.
		if skill.Scope != "" && !slices.Contains(types.ValidScopes(), skill.Scope) {
			return at(loc.skillKeyLine(i, "scope"), "skill %q has invalid scope %q (valid: %s)",
				skill.Name, skill.Scope, strings.Join(types.ValidScopes(), ", "))
		}

		if err := validateTargets(
			fmt.Sprintf("skills[%d].targets", i),
			skill.Targets,
			loc.skillKeyLine(i, "targets"),
		); err != nil {
			return err
		}
	}

	return nil
}

// validateTargets rejects agent names skmgr does not know.
//
// An unrecognized target used to be skipped with a note during verification,
// which meant a typo like "curser" produced an install that looked successful
// and did nothing.
func validateTargets(field string, targets []string, line int) error {
	known := types.SupportedAgentNames()

	for _, target := range targets {
		if slices.Contains(known, target) {
			continue
		}

		message := fmt.Sprintf("%s has unknown agent %q (known: %s)",
			field, target, strings.Join(known, ", "))

		if suggestion := closestAgent(target, known); suggestion != "" {
			message += fmt.Sprintf("; did you mean %q?", suggestion)
		}

		return at(line, "%s", message)
	}

	return nil
}

// closestAgent guesses which agent a misspelling meant, and returns "" when
// nothing is close enough to be worth suggesting.
func closestAgent(value string, known []string) string {
	lowered := strings.ToLower(value)

	best, bestDistance := "", 0
	for _, candidate := range known {
		distance := editDistance(lowered, strings.ToLower(candidate))

		// Two edits is close enough to be a typo; beyond that a suggestion is
		// more likely to mislead than help.
		if distance > 2 {
			continue
		}
		if best == "" || distance < bestDistance {
			best, bestDistance = candidate, distance
		}
	}

	return best
}

// editDistance is the Levenshtein distance between two strings.
func editDistance(a string, b string) int {
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)

	for j := range previous {
		previous[j] = j
	}

	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			current[j] = min(
				previous[j]+1,
				current[j-1]+1,
				previous[j-1]+cost,
			)
		}
		copy(previous, current)
	}

	return previous[len(b)]
}
