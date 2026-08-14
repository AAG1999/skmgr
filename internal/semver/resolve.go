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

package semver

import (
	"fmt"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"
)

// Tag pairs a git tag with the version parsed from it.
//
// The original string is kept because that is what has to be checked out: a
// repository may tag "v1.2.0" or "1.2.0", and reconstructing it from the parsed
// version would guess wrong half the time.
type Tag struct {
	Name    string
	Version *semver.Version
}

// ErrNoVersions reports that a repository has no tags that parse as semantic
// versions, so a range cannot be resolved against it.
type ErrNoVersions struct {
	// Tags are whatever tags the repository does have, for the error message.
	Tags []string
}

func (e *ErrNoVersions) Error() string {
	if len(e.Tags) == 0 {
		return "the repository has no tags, so a version range cannot be resolved; " +
			"use a branch name or a commit SHA as the ref instead"
	}

	shown := e.Tags
	if len(shown) > 5 {
		shown = shown[:5]
	}

	return fmt.Sprintf(
		"none of the repository's tags are semantic versions (found %s), so a "+
			"version range cannot be resolved; use one of those tags, a branch "+
			"name, or a commit SHA as the ref instead",
		strings.Join(shown, ", "))
}

// ErrNoMatch reports that tags exist but none satisfies the constraint.
type ErrNoMatch struct {
	Constraint string
	Available  []string
}

func (e *ErrNoMatch) Error() string {
	shown := e.Available
	if len(shown) > 5 {
		shown = shown[:5]
	}

	return fmt.Sprintf("no tag satisfies %q; the newest available are %s",
		e.Constraint, strings.Join(shown, ", "))
}

// ParseTags turns raw tag names into versions, discarding the ones that are not
// semantic versions. A repository is free to carry tags like "nightly" or
// "release-candidate" alongside real versions, and those simply do not
// participate in range resolution.
//
// The result is sorted newest first.
func ParseTags(tags []string) []Tag {
	var parsed []Tag

	for _, name := range tags {
		version, err := semver.NewVersion(strings.TrimSpace(name))
		if err != nil {
			continue
		}
		parsed = append(parsed, Tag{Name: name, Version: version})
	}

	sort.Slice(parsed, func(i, j int) bool {
		return parsed[i].Version.GreaterThan(parsed[j].Version)
	})

	return parsed
}

// Resolve returns the newest tag satisfying the constraint.
//
// Prereleases are excluded unless the constraint itself names one, which is the
// usual convention: someone asking for "^1.2.0" wants a release, and someone
// asking for "^1.2.0-beta.1" has opted in.
func Resolve(tags []string, constraint string) (string, error) {
	parsed := ParseTags(tags)
	if len(parsed) == 0 {
		return "", &ErrNoVersions{Tags: tags}
	}

	trimmed := strings.TrimSpace(constraint)

	if strings.EqualFold(trimmed, Latest) {
		for _, tag := range parsed {
			if tag.Version.Prerelease() == "" {
				return tag.Name, nil
			}
		}
		return "", &ErrNoMatch{Constraint: Latest, Available: names(parsed)}
	}

	c, err := semver.NewConstraint(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid version range %q: %w", constraint, err)
	}

	wantsPrerelease := strings.Contains(trimmed, "-")

	for _, tag := range parsed {
		if tag.Version.Prerelease() != "" && !wantsPrerelease {
			continue
		}
		if c.Check(tag.Version) {
			return tag.Name, nil
		}
	}

	return "", &ErrNoMatch{Constraint: trimmed, Available: names(parsed)}
}

// Newest returns the highest stable tag, ignoring any constraint. It backs
// `--latest` and the "latest overall" column of `skmgr outdated`.
func Newest(tags []string) (string, error) {
	return Resolve(tags, Latest)
}

// names lists tag names newest first, for error messages.
func names(parsed []Tag) []string {
	out := make([]string, 0, len(parsed))
	for _, tag := range parsed {
		out = append(out, tag.Name)
	}
	return out
}
