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
	"os/exec"
	"strings"

	"github.com/AAG1999/skmgr/internal/semver"
)

// listTags returns every tag in a cached clone.
//
// The tags come from the clone rather than from `git ls-remote`, because the
// clone is fetched with --tags on every run and is therefore already as current
// as a remote query would be. Reading it locally costs no round trip, needs no
// freshness heuristic, and is the only option that works under --offline.
func listTags(repoDir string) ([]string, error) {
	cmd := exec.Command("git", "tag", "--list")
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("listing tags: %w", err)
	}

	var tags []string
	for _, line := range strings.Split(string(out), "\n") {
		if tag := strings.TrimSpace(line); tag != "" {
			tags = append(tags, tag)
		}
	}

	return tags, nil
}

// resolveRange turns a version range into the concrete tag it selects.
func resolveRange(repoDir string, constraint string, latest bool) (string, error) {
	tags, err := listTags(repoDir)
	if err != nil {
		return "", err
	}

	if latest {
		constraint = semver.Latest
	}

	tag, err := semver.Resolve(tags, constraint)
	if err != nil {
		return "", err
	}

	return tag, nil
}

// ListTags returns the tags of a source repository, cloning or fetching first
// if the cache does not already hold it.
//
// It exists for `skmgr outdated`, which needs to know what versions are
// available without materializing any of them.
func ListTags(source string, cacheDir string, opts FetchOptions) ([]string, error) {
	source = CloneURL(source)
	cache := newGitCache(cacheDir, source)

	if err := cache.withRepo(source, opts); err != nil {
		return nil, err
	}

	return listTags(cache.repoDir)
}
