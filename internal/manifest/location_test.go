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
	"strings"
	"testing"
)

// A manifest with thirty skills makes "skills[17]" a counting exercise. The
// line number is what the user actually needs to fix the file.
func TestParseBytes_ErrorsPointAtTheLine(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		wantLine string
		wantText string
	}{
		{
			name: "unknown agent in a skill",
			manifest: `version: "1"
name: demo
targets:
  - cursor
skills:
  - source: ./one
    path: skills/one
  - source: ./two
    path: skills/two
    targets:
      - curser
`,
			wantLine: "line 10",
			wantText: "did you mean \"cursor\"",
		},
		{
			name: "unknown agent at the top level",
			manifest: `version: "1"
name: demo
targets:
  - claud-code
`,
			wantLine: "line 3",
			wantText: "unknown agent",
		},
		{
			name: "invalid type",
			manifest: `version: "1"
name: demo
skills:
  - source: ./one
    path: skills/one
    type: guideline
`,
			wantLine: "line 6",
			wantText: "invalid type",
		},
		{
			name: "invalid scope",
			manifest: `version: "1"
name: demo
skills:
  - source: ./one
    path: skills/one
    scope: everywhere
`,
			wantLine: "line 6",
			wantText: "invalid scope",
		},
		{
			name: "missing source",
			manifest: `version: "1"
name: demo
skills:
  - source: ./one
    path: skills/one
  - path: skills/two
`,
			wantLine: "line 6",
			wantText: "missing 'source'",
		},
		{
			name: "duplicate name",
			manifest: `version: "1"
name: demo
skills:
  - source: ./one
    path: skills/shared
  - source: ./two
    path: other/shared
`,
			wantLine: "line 6",
			wantText: "duplicate skill name",
		},
		{
			name: "a source git would read as an option",
			manifest: `version: "1"
name: demo
skills:
  - source: --upload-pack=touch /tmp/pwned
    name: hostile
`,
			wantLine: "line 4",
			wantText: "git would read as an option",
		},
		{
			name: "a ref git would read as an option",
			manifest: `version: "1"
name: demo
skills:
  - source: ./one
    path: skills/one
    ref: --exec=sh
`,
			wantLine: "line 6",
			wantText: "git would read as an option",
		},
		{
			name: "names differing only in case",
			manifest: `version: "1"
name: demo
skills:
  - source: ./one
    name: Greeter
    path: skills/a
  - source: ./two
    name: greeter
    path: skills/b
`,
			wantLine: "line 7",
			wantText: "differ only in case",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseBytes([]byte(tt.manifest))
			if err == nil {
				t.Fatal("expected a validation error")
			}

			if !strings.Contains(err.Error(), tt.wantLine) {
				t.Errorf("error should name %s, got: %v", tt.wantLine, err)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("error should explain the problem (%q), got: %v", tt.wantText, err)
			}
		})
	}
}

// The top-level name is reported against its own line when it is present but
// blank, and without a line when the key was never written at all.
func TestParseBytes_MissingNameLocation(t *testing.T) {
	blank := `version: "1"
name: ""
`
	err := parseError(t, blank)
	if !strings.Contains(err, "line 2") {
		t.Errorf("a blank name should point at its line, got: %s", err)
	}

	absent := `version: "1"
targets:
  - cursor
`
	err = parseError(t, absent)
	if strings.Contains(err, "line") {
		t.Errorf("an absent key has no line to point at, got: %s", err)
	}
	if !strings.Contains(err, "'name' is required") {
		t.Errorf("error should still say what is missing, got: %s", err)
	}
}

// Line lookups run against a document that failed to parse as a node tree only
// in odd cases, but they must not panic when they do.
func TestLocations_ToleratesUnusableInput(t *testing.T) {
	for _, input := range []string{"", "just a string", "- a\n- list\n", ":\n"} {
		loc := newLocations([]byte(input))

		if line := loc.topLine("name"); line != 0 {
			t.Errorf("topLine(%q) = %d, want 0", input, line)
		}
		if line := loc.skillLine(0); line != 0 {
			t.Errorf("skillLine(%q) = %d, want 0", input, line)
		}
		if line := loc.skillKeyLine(0, "path"); line != 0 {
			t.Errorf("skillKeyLine(%q) = %d, want 0", input, line)
		}
	}
}

func TestLocations_NilIsSafe(t *testing.T) {
	var loc *locations

	if loc.topLine("name") != 0 || loc.skillLine(0) != 0 || loc.skillKeyLine(0, "path") != 0 {
		t.Error("a nil locations should report no lines rather than panic")
	}
}

// parseError returns the message from parsing a manifest that must fail.
func parseError(t *testing.T, manifest string) string {
	t.Helper()

	_, err := ParseBytes([]byte(manifest))
	if err == nil {
		t.Fatal("expected a validation error")
	}
	return err.Error()
}
