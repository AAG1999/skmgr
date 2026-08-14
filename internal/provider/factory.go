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
	"strings"
)

// GitPrefix forces a source to be treated as a git repository.
//
// A path on disk is otherwise read as a plain directory, which is usually what
// someone pointing at a folder means. It is the wrong reading for a local clone
// or mirror, where the point is to check out a ref: without git treatment,
// `ref` and version ranges have nothing to act on and are silently ignored.
// Prefixing the source with "git::" says which one you meant.
const GitPrefix = "git::"

// GetProvider returns the appropriate Provider implementation based on the source URL.
func GetProvider(source string) Provider {
	// An explicit git:: prefix wins over everything below.
	if strings.HasPrefix(source, GitPrefix) {
		return &GitProvider{}
	}

	// If it explicitly starts with file://, it's local.
	if strings.HasPrefix(source, "file://") {
		return &LocalProvider{}
	}

	// If it starts with / or ./ or ../, it's local.
	if strings.HasPrefix(source, "/") || strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") {
		return &LocalProvider{}
	}

	// For Windows, check for drive letters (e.g., C:\)
	if len(source) > 2 && source[1] == ':' && (source[2] == '\\' || source[2] == '/') {
		return &LocalProvider{}
	}

	// A bare relative path such as "vendor/skills" carries no marker saying it
	// is local, and treating it as a git URL produces a confusing clone failure
	// for a directory sitting right there. Checking the disk resolves it.
	if !looksRemote(source) && isDirectory(source) {
		return &LocalProvider{}
	}

	// Otherwise, assume it's a git URL (https://, git@, etc.)
	return &GitProvider{}
}

// looksRemote reports whether a source is unmistakably a remote URL.
//
// These forms are never checked against the disk, so a directory that happens
// to share a name with a host cannot hijack a real remote.
func looksRemote(source string) bool {
	return strings.Contains(source, "://") ||
		strings.HasPrefix(source, "git@") ||
		strings.HasPrefix(source, "ssh://") ||
		strings.Contains(source, ".git")
}

// isDirectory reports whether a source names a directory on disk, resolved
// relative to the working directory.
func isDirectory(source string) bool {
	info, err := os.Stat(filepath.FromSlash(source))
	return err == nil && info.IsDir()
}

// CloneURL strips the git:: marker, which is skmgr's notation and means nothing
// to git itself.
func CloneURL(source string) string {
	return strings.TrimPrefix(source, GitPrefix)
}
