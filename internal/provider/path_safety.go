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
	"fmt"
	"path/filepath"
	"strings"
)

// resolveWithinBase joins rel onto base and verifies the result stays inside
// base. A manifest is user-supplied data, so a path like "../../.ssh" must not
// be able to pull arbitrary host directories into the install tree.
func resolveWithinBase(base string, rel string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("resolving base path: %w", err)
	}

	joined := filepath.Join(absBase, filepath.FromSlash(rel))
	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("resolving path %q: %w", rel, err)
	}

	if !isWithin(absBase, absJoined) {
		return "", fmt.Errorf("path %q escapes the source root", rel)
	}

	return absJoined, nil
}

// isWithin reports whether candidate is base itself or nested underneath it.
func isWithin(base string, candidate string) bool {
	if candidate == base {
		return true
	}
	prefix := base
	if !strings.HasSuffix(prefix, string(filepath.Separator)) {
		prefix += string(filepath.Separator)
	}
	return strings.HasPrefix(candidate, prefix)
}

// filterWithinBase drops any glob match that resolved outside base. Go's
// filepath.Glob happily expands patterns containing "..", so matches have to be
// re-checked after expansion rather than trusting the pattern alone.
func filterWithinBase(base string, matches []string) ([]string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return nil, fmt.Errorf("resolving base path: %w", err)
	}

	kept := make([]string, 0, len(matches))
	for _, match := range matches {
		absMatch, err := filepath.Abs(match)
		if err != nil {
			continue
		}
		if isWithin(absBase, absMatch) {
			kept = append(kept, absMatch)
		}
	}
	return kept, nil
}
