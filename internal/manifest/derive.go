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

package manifest

import (
	"fmt"
	"path"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
)

// IsWildcard reports whether a path uses glob syntax. Wildcard entries expand
// into several skills, so their names are generated per match by the resolver
// rather than derived here.
func IsWildcard(p string) bool {
	return strings.ContainsAny(p, "*?[")
}

// DeriveName computes the local handle for a skill when the manifest does not
// spell one out.
//
// The name is only an alias: identity is source + path + ref. Requiring it
// added noise to every entry, so it is derived from whichever part of the
// declaration already describes the skill.
//
//	path set and not a wildcard  -> last segment of path
//	otherwise                    -> repository name from source
//
// A candidate that would not survive [types.ValidateName] is skipped rather
// than returned, because the name becomes a directory and every caller assumes
// what it gets back is usable as one.
func DeriveName(source string, p string) (string, error) {
	var candidates []string
	if p != "" && !IsWildcard(p) {
		candidates = append(candidates, lastSegment(p))
	}
	candidates = append(candidates, repoNameFromSource(source))

	// Report why the first real candidate was rejected. "cannot derive a name"
	// alone leaves the user guessing which part of their entry was the problem.
	var reason error
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if err := types.ValidateName(candidate); err != nil {
			if reason == nil {
				reason = err
			}
			continue
		}
		return candidate, nil
	}

	if reason != nil {
		return "", fmt.Errorf("cannot derive a usable name from source %q and path %q: %w; set 'name' explicitly", source, p, reason)
	}
	return "", fmt.Errorf("cannot derive a name from source %q and path %q; set 'name' explicitly", source, p)
}

// lastSegment returns the final component of a slash- or backslash-separated
// path, ignoring trailing separators.
//
// It returns "" when no usable component exists, so the caller falls back to
// the source rather than deriving a name that cannot be a directory.
func lastSegment(p string) string {
	normalized := strings.ReplaceAll(p, `\`, "/")
	normalized = strings.Trim(normalized, "/")
	if normalized == "" {
		return ""
	}

	base := path.Base(normalized)
	if base == "." || base == ".." || base == "/" {
		return ""
	}

	// A segment of only whitespace is a directory name no one can work with,
	// and passing it on produces a "name must not be empty" complaint about a
	// name the user never wrote. Treat it as no name at all.
	base = strings.TrimSpace(base)
	if base == "" {
		return ""
	}

	return base
}

// repoNameFromSource extracts a repository name from a git URL or local path.
// It handles https URLs, scp-style SSH remotes (git@host:owner/repo.git),
// file:// URLs and plain filesystem paths.
func repoNameFromSource(source string) string {
	s := strings.TrimSpace(source)
	if s == "" {
		return ""
	}

	s = strings.TrimPrefix(s, "file://")

	// Drop any URL query or fragment before looking at the path.
	if i := strings.IndexAny(s, "?#"); i >= 0 {
		s = s[:i]
	}

	// scp-style remotes have no scheme; everything after the colon is the path.
	if !strings.Contains(s, "://") {
		if i := strings.LastIndex(s, ":"); i >= 0 && !isWindowsDriveColon(s, i) {
			s = s[i+1:]
		}
	} else {
		s = s[strings.Index(s, "://")+3:]
		// Strip the host portion.
		if i := strings.Index(s, "/"); i >= 0 {
			s = s[i+1:]
		} else {
			return ""
		}
	}

	name := lastSegment(s)
	return strings.TrimSuffix(name, ".git")
}

// isWindowsDriveColon reports whether the colon at index i is the drive
// separator in a path like C:\repos\thing.
func isWindowsDriveColon(s string, i int) bool {
	return i == 1 && len(s) > 2 && (s[2] == '\\' || s[2] == '/')
}
