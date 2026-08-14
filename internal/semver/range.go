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

// Package semver resolves semantic-version ranges against a repository's tags.
//
// skmgr has no registry, so a git tag is the only thing resembling a published
// version. A manifest may therefore ask for a range (`^1.2.0`) or for `latest`
// instead of naming one tag, and resolution picks the best tag that satisfies
// it.
package semver

import "strings"

// Latest is the ref that asks for the newest release, whatever it is.
const Latest = "latest"

// IsRange reports whether a ref should be resolved as a version range rather
// than used as a literal git ref.
//
// The test is deliberately conservative, because a ref is also allowed to be a
// branch or tag name and misreading one as a range would silently install
// something the user did not ask for. Only unambiguous forms count:
//
//   - the word "latest"
//   - a leading operator: ^ ~ > < =
//   - a wildcard in a version position: *, 1.x, 1.2.X
//   - a compound constraint: "1.2 - 1.4", ">=1.0 <2.0", "1.x || 2.x"
//
// Anything else — including a bare "v1.2.3", "1.2.3", "main", or "release-2" —
// is a literal ref. In particular a plain version is treated as an exact tag,
// since that is what someone writing `ref: v1.2.3` means.
func IsRange(ref string) bool {
	trimmed := strings.TrimSpace(ref)
	if trimmed == "" {
		return false
	}

	if strings.EqualFold(trimmed, Latest) {
		return true
	}

	switch trimmed[0] {
	case '^', '~', '>', '<', '=':
		return true
	}

	if strings.Contains(trimmed, "||") || strings.Contains(trimmed, " - ") {
		return true
	}

	// A space-separated list is an AND of constraints, but only if the parts
	// look like constraints rather than a branch name that happens to have a
	// space in it.
	if fields := strings.Fields(trimmed); len(fields) > 1 {
		for _, field := range fields {
			if !IsRange(field) && !hasVersionWildcard(field) {
				return false
			}
		}
		return true
	}

	return hasVersionWildcard(trimmed)
}

// hasVersionWildcard reports whether a single token uses a wildcard in place of
// a version component, as in "*", "1.x" or "1.2.*".
func hasVersionWildcard(token string) bool {
	trimmed := strings.TrimPrefix(strings.TrimSpace(token), "v")
	if trimmed == "" {
		return false
	}

	if trimmed == "*" || strings.EqualFold(trimmed, "x") {
		return true
	}

	parts := strings.Split(trimmed, ".")
	if len(parts) < 2 {
		return false
	}

	sawWildcard := false

	for _, part := range parts {
		switch {
		case part == "*" || strings.EqualFold(part, "x"):
			sawWildcard = true
		case isNumeric(part):
			// A numeric component is fine either way.
		default:
			// Anything else means this is not a version pattern at all, so a
			// branch named "feature/x.y" is not mistaken for one.
			return false
		}
	}

	return sawWildcard
}

// isNumeric reports whether a string is a non-empty run of digits.
func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
