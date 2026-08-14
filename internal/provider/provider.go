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

// Package provider abstracts fetching skills from various sources (git, local).
package provider

import "github.com/AAG1999/skmgr/internal/types"

// FetchResult contains the outcome of fetching a skill.
type FetchResult struct {
	// SourceDirs contains the absolute paths to the fetched skill contents.
	// For git, this is inside the skmgr cache. For local, it's the target directory itself.
	// If a glob path was used, this may contain multiple directories.
	SourceDirs []string

	// CommitSHA is the resolved git commit, if applicable.
	// Empty for local sources.
	CommitSHA string

	// ResolvedRef is the concrete ref a version range resolved to, such as the
	// tag "v1.4.2" for the range "^1.2.0". It is empty when the manifest named
	// a literal ref, since there is nothing to record beyond what it already
	// says.
	ResolvedRef string
}

// FetchOptions tunes how a provider obtains a source.
type FetchOptions struct {
	// Offline forbids all network access. A git source must already be in the
	// cache and the requested ref must resolve locally, otherwise the fetch
	// fails rather than silently reaching out.
	Offline bool

	// Latest ignores the declared version range and takes the newest release
	// tag instead. It backs `skmgr update --latest`, which is how a user asks
	// to cross a major version deliberately.
	Latest bool
}

// Provider defines the interface for fetching skills from a source.
type Provider interface {
	// Fetch retrieves the skill and returns the local path to its contents.
	// cacheDir is the global ~/.skmgr/cache directory.
	Fetch(skill types.SkillDependency, cacheDir string, opts FetchOptions) (FetchResult, error)
}
