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
	"errors"
	"strings"
	"testing"
)

var repoTags = []string{
	"v1.0.0",
	"v1.1.0",
	"v1.2.0",
	"v1.2.1",
	"v2.0.0",
	"v2.1.0-beta.1",
	"nightly",
	"release-candidate",
}

func TestResolve(t *testing.T) {
	cases := []struct {
		constraint string
		want       string
		why        string
	}{
		{constraint: "^1.0.0", want: "v1.2.1", why: "caret allows minor and patch"},
		{constraint: "^1.1.0", want: "v1.2.1", why: "caret from a later minor"},
		{constraint: "~1.1.0", want: "v1.1.0", why: "tilde pins the minor"},
		{constraint: "~1.2.0", want: "v1.2.1", why: "tilde allows patch"},
		{constraint: ">=1.0.0", want: "v2.0.0", why: "open upper bound reaches v2"},
		{constraint: ">=1.0.0 <2.0.0", want: "v1.2.1", why: "compound constraint"},
		{constraint: "1.x", want: "v1.2.1", why: "wildcard minor"},
		{constraint: "1.2.x", want: "v1.2.1", why: "wildcard patch"},
		{constraint: "*", want: "v2.0.0", why: "any stable version"},
		{constraint: "latest", want: "v2.0.0", why: "newest stable"},
		{constraint: "=1.1.0", want: "v1.1.0", why: "exact"},
		{constraint: "1.0.0 - 1.2.0", want: "v1.2.0", why: "hyphen range"},
		{constraint: "1.x || 2.x", want: "v2.0.0", why: "union picks the highest"},
	}

	for _, tc := range cases {
		t.Run(tc.constraint, func(t *testing.T) {
			got, err := Resolve(repoTags, tc.constraint)
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tc.constraint, err)
			}
			if got != tc.want {
				t.Errorf("Resolve(%q) = %q, want %q (%s)", tc.constraint, got, tc.want, tc.why)
			}
		})
	}
}

// A prerelease is only a candidate when the constraint asks for one. Otherwise
// following "^2.0.0" would quietly move a team onto a beta.
func TestResolve_PrereleaseHandling(t *testing.T) {
	tags := []string{"v1.0.0", "v2.0.0", "v2.1.0-beta.1", "v2.1.0-beta.2"}

	cases := []struct {
		name       string
		constraint string
		want       string
		wantErr    bool
	}{
		{name: "stable range skips prereleases", constraint: "^2.0.0", want: "v2.0.0"},
		{name: "latest skips prereleases", constraint: "latest", want: "v2.0.0"},
		{name: "star skips prereleases", constraint: "*", want: "v2.0.0"},
		{name: "explicit prerelease opts in", constraint: ">=2.1.0-beta.1", want: "v2.1.0-beta.2"},
		{name: "prerelease range picks the newest beta", constraint: "^2.1.0-beta.1", want: "v2.1.0-beta.2"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tags, tc.constraint)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%q): %v", tc.constraint, err)
			}
			if got != tc.want {
				t.Errorf("Resolve(%q) = %q, want %q", tc.constraint, got, tc.want)
			}
		})
	}
}

// Only prereleases exist, and the constraint does not ask for one.
func TestResolve_LatestWithOnlyPrereleases(t *testing.T) {
	_, err := Resolve([]string{"v1.0.0-rc.1"}, "latest")

	var noMatch *ErrNoMatch
	if !errors.As(err, &noMatch) {
		t.Fatalf("expected ErrNoMatch, got %v", err)
	}
}

func TestResolve_NoSemverTags(t *testing.T) {
	_, err := Resolve([]string{"nightly", "release-candidate"}, "^1.0.0")

	var noVersions *ErrNoVersions
	if !errors.As(err, &noVersions) {
		t.Fatalf("expected ErrNoVersions, got %v", err)
	}

	// The message has to say what to do instead, since the user's manifest is
	// simply incompatible with this repository.
	for _, want := range []string{"nightly", "branch name", "commit SHA"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error should mention %q, got: %s", want, err)
		}
	}
}

func TestResolve_NoTagsAtAll(t *testing.T) {
	_, err := Resolve(nil, "^1.0.0")

	var noVersions *ErrNoVersions
	if !errors.As(err, &noVersions) {
		t.Fatalf("expected ErrNoVersions, got %v", err)
	}
	if !strings.Contains(err.Error(), "no tags") {
		t.Errorf("error should say the repository has no tags, got: %s", err)
	}
}

func TestResolve_NothingSatisfiesTheRange(t *testing.T) {
	_, err := Resolve(repoTags, "^9.0.0")

	var noMatch *ErrNoMatch
	if !errors.As(err, &noMatch) {
		t.Fatalf("expected ErrNoMatch, got %v", err)
	}
	if !strings.Contains(err.Error(), "v2.0.0") {
		t.Errorf("error should list what is available, got: %s", err)
	}
}

func TestResolve_InvalidConstraint(t *testing.T) {
	_, err := Resolve(repoTags, "^^1.0")
	if err == nil {
		t.Fatal("expected an error for a malformed range")
	}
	if !strings.Contains(err.Error(), "invalid version range") {
		t.Errorf("error should name the problem, got: %s", err)
	}
}

func TestParseTags_SortsNewestFirstAndDropsNonVersions(t *testing.T) {
	parsed := ParseTags([]string{"v1.0.0", "nightly", "v2.0.0", "v1.5.0"})

	if len(parsed) != 3 {
		t.Fatalf("expected 3 version tags, got %d", len(parsed))
	}

	want := []string{"v2.0.0", "v1.5.0", "v1.0.0"}
	for i, tag := range parsed {
		if tag.Name != want[i] {
			t.Errorf("position %d = %q, want %q", i, tag.Name, want[i])
		}
	}
}

// The tag string is what gets checked out, so the original spelling has to
// survive parsing rather than being rebuilt from the version.
func TestParseTags_KeepsTheOriginalSpelling(t *testing.T) {
	parsed := ParseTags([]string{"1.0.0", "v2.0.0"})

	if parsed[0].Name != "v2.0.0" {
		t.Errorf("expected the v-prefixed spelling to survive, got %q", parsed[0].Name)
	}
	if parsed[1].Name != "1.0.0" {
		t.Errorf("expected the bare spelling to survive, got %q", parsed[1].Name)
	}
}

func TestNewest(t *testing.T) {
	got, err := Newest(repoTags)
	if err != nil {
		t.Fatal(err)
	}
	if got != "v2.0.0" {
		t.Errorf("Newest = %q, want v2.0.0", got)
	}
}
