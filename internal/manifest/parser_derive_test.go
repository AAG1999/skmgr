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
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestDeriveName(t *testing.T) {
	tests := []struct {
		name   string
		source string
		path   string
		want   string
	}{
		{"path wins over source", "https://github.com/acme/tools.git", "skills/formatter", "formatter"},
		{"nested path", "https://github.com/acme/tools.git", "a/b/c/deep-skill", "deep-skill"},
		{"trailing slash", "https://github.com/acme/tools.git", "skills/formatter/", "formatter"},
		{"https source, no path", "https://github.com/acme/tools.git", "", "tools"},
		{"https source without .git", "https://github.com/acme/tools", "", "tools"},
		{"ssh scp-style source", "git@github.com:acme/tools.git", "", "tools"},
		{"ssh url source", "ssh://git@github.com/acme/tools.git", "", "tools"},
		{"file url source", "file:///home/me/my-skills", "", "my-skills"},
		{"plain local path", "/home/me/my-skills", "", "my-skills"},
		{"relative local path", "./vendor/shared-skills", "", "shared-skills"},
		{"source with query", "https://github.com/acme/tools.git?ref=x", "", "tools"},
		{"backslash path", "https://github.com/acme/tools.git", `skills\formatter`, "formatter"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DeriveName(tc.source, tc.path)
			if err != nil {
				t.Fatalf("DeriveName(%q, %q) returned error: %v", tc.source, tc.path, err)
			}
			if got != tc.want {
				t.Errorf("DeriveName(%q, %q) = %q, want %q", tc.source, tc.path, got, tc.want)
			}
		})
	}
}

func TestDeriveName_FailsWithoutUsableSource(t *testing.T) {
	if _, err := DeriveName("", ""); err == nil {
		t.Error("DeriveName(\"\", \"\") should have returned an error")
	}
}

// Found by FuzzDeriveName. Trimming only slashes left " " as the last segment,
// which was returned as a name and then failed far away with "name must not be
// empty" — about a name the user never wrote. An unusable segment is skipped so
// the source can answer instead.
func TestDeriveName_SkipsAnUnusablePathSegment(t *testing.T) {
	got, err := DeriveName("0", " ")
	if err != nil {
		t.Fatalf("DeriveName() returned error: %v", err)
	}
	if got != "0" {
		t.Errorf("DeriveName(%q, %q) = %q, want the source to be used instead", "0", " ", got)
	}
}

// Found by FuzzDeriveName. A quote is legal in a Linux path but not in a name
// skmgr joins onto agent directories on every platform. With no source to fall
// back to there is no name to be had, and saying so beats inventing one.
func TestDeriveName_FailsWhenNoCandidateIsUsable(t *testing.T) {
	got, err := DeriveName("", `"00000`)
	if err == nil {
		t.Fatalf("DeriveName(%q, %q) = %q, want an error", "", `"00000`, got)
	}
	if !strings.Contains(err.Error(), "set 'name' explicitly") {
		t.Errorf("error %q does not tell the user how to proceed", err)
	}
	// The reason the candidate was rejected is the actionable part.
	if !strings.Contains(err.Error(), "invalid in a directory name") {
		t.Errorf("error %q does not say what was wrong with the derived name", err)
	}
}

// Whatever derivation returns is used as a directory name without further
// checks, so the guarantee has to hold for every accepted input.
func TestDeriveName_ResultIsAlwaysAValidName(t *testing.T) {
	sources := []string{"", "0", "https://github.com/acme/tools.git", "git@github.com:acme/tools.git", "./local"}
	paths := []string{"", " ", "  ", `"quote`, "skills/ok", "skills/", "..", ".", "skills/*", "a:b"}

	for _, source := range sources {
		for _, p := range paths {
			got, err := DeriveName(source, p)
			if err != nil {
				continue
			}
			if err := types.ValidateName(got); err != nil {
				t.Errorf("DeriveName(%q, %q) = %q, which is not a usable name: %v", source, p, got, err)
			}
		}
	}
}

// A wildcard path expands into many skills, so it cannot name a single one.
// Derivation must fall back to the repository name instead.
func TestDeriveName_WildcardFallsBackToSource(t *testing.T) {
	got, err := DeriveName("https://github.com/acme/tools.git", "skills/*")
	if err != nil {
		t.Fatalf("DeriveName() returned error: %v", err)
	}
	if got != "tools" {
		t.Errorf("DeriveName() = %q, want %q", got, "tools")
	}
}

func TestIsWildcard(t *testing.T) {
	wildcards := []string{"skills/*", "skills/?ool", "skills/[ab]c", "*"}
	for _, p := range wildcards {
		if !IsWildcard(p) {
			t.Errorf("IsWildcard(%q) = false, want true", p)
		}
	}

	literals := []string{"skills/formatter", "", "a/b/c"}
	for _, p := range literals {
		if IsWildcard(p) {
			t.Errorf("IsWildcard(%q) = true, want false", p)
		}
	}
}

func TestParseBytes_DerivesNameFromPath(t *testing.T) {
	doc := `
name: test
skills:
  - source: https://github.com/acme/tools.git
    path: skills/formatter
`
	m, err := ParseBytes([]byte(doc))
	if err != nil {
		t.Fatalf("ParseBytes() returned error: %v", err)
	}
	if m.Skills[0].Name != "formatter" {
		t.Errorf("Name = %q, want %q", m.Skills[0].Name, "formatter")
	}
}

func TestParseBytes_DerivesNameFromSource(t *testing.T) {
	doc := `
name: test
skills:
  - source: https://github.com/acme/my-tools.git
`
	m, err := ParseBytes([]byte(doc))
	if err != nil {
		t.Fatalf("ParseBytes() returned error: %v", err)
	}
	if m.Skills[0].Name != "my-tools" {
		t.Errorf("Name = %q, want %q", m.Skills[0].Name, "my-tools")
	}
}

// An explicit name is a deliberate alias and must not be overwritten, even
// when it disagrees with the path.
func TestParseBytes_ExplicitNameOverridesDerivation(t *testing.T) {
	doc := `
name: test
skills:
  - name: skill-not-creator
    source: https://github.com/acme/tools.git
    path: skills/skill-creator
`
	m, err := ParseBytes([]byte(doc))
	if err != nil {
		t.Fatalf("ParseBytes() returned error: %v", err)
	}
	if m.Skills[0].Name != "skill-not-creator" {
		t.Errorf("Name = %q, want the explicit override %q", m.Skills[0].Name, "skill-not-creator")
	}
}

func TestParseBytes_WildcardNameLeftEmpty(t *testing.T) {
	doc := `
name: test
skills:
  - source: https://github.com/acme/tools.git
    path: skills/*
`
	m, err := ParseBytes([]byte(doc))
	if err != nil {
		t.Fatalf("ParseBytes() returned error: %v", err)
	}
	if m.Skills[0].Name != "" {
		t.Errorf("Name = %q, want empty so the resolver can name each match", m.Skills[0].Name)
	}
}

// Two skills deriving to the same handle must be reported rather than silently
// overwriting one another in .agents/.
func TestParseBytes_DerivedCollisionIsRejected(t *testing.T) {
	doc := `
name: test
skills:
  - source: https://github.com/acme/one.git
    path: skills/formatter
  - source: https://github.com/acme/two.git
    path: skills/formatter
`
	_, err := ParseBytes([]byte(doc))
	if err == nil {
		t.Fatal("ParseBytes() accepted two skills deriving to the same name, want error")
	}
	if !strings.Contains(err.Error(), "duplicate skill name") {
		t.Errorf("error = %q, want it to report a duplicate name", err.Error())
	}
}

func TestParseBytes_MissingSourceIsReported(t *testing.T) {
	doc := `
name: test
skills:
  - path: skills/formatter
`
	_, err := ParseBytes([]byte(doc))
	if err == nil {
		t.Fatal("ParseBytes() accepted a skill with no source, want error")
	}
	if !strings.Contains(err.Error(), "source") {
		t.Errorf("error = %q, want it to mention the missing source", err.Error())
	}
}

func TestParse_DeriveFixtures(t *testing.T) {
	tests := []struct {
		file      string
		wantNames []string
	}{
		{"derive_from_path.yml", []string{"formatter", "linter"}},
		{"derive_from_source.yml", []string{"shared-skills"}},
		{"override_alias.yml", []string{"fmt"}},
	}

	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			m, err := Parse(filepath.Join("..", "..", "testdata", "manifests", tc.file))
			if err != nil {
				t.Fatalf("Parse(%s) returned error: %v", tc.file, err)
			}
			if len(m.Skills) != len(tc.wantNames) {
				t.Fatalf("got %d skills, want %d", len(m.Skills), len(tc.wantNames))
			}
			for i, want := range tc.wantNames {
				if m.Skills[i].Name != want {
					t.Errorf("skills[%d].Name = %q, want %q", i, m.Skills[i].Name, want)
				}
			}
		})
	}
}
