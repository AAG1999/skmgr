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
	"strings"
	"testing"
)

// A skill name becomes a directory component under .agents/ and under each
// agent directory, so anything that could escape the tree must be rejected
// at parse time.
func TestParseBytes_RejectsUnsafeNames(t *testing.T) {
	unsafe := []struct {
		name  string
		value string
	}{
		{"parent traversal", "../etc/passwd"},
		{"bare parent", ".."},
		{"current dir", "."},
		{"embedded traversal", "foo/../../bar"},
		{"forward slash", "foo/bar"},
		{"backslash", `foo\bar`},
		{"colon", "foo:bar"},
		{"wildcard", "foo*"},
		{"question mark", "foo?"},
		{"pipe", "foo|bar"},
		{"windows reserved", "CON"},
		{"windows reserved lower", "nul"},
	}

	for _, tc := range unsafe {
		t.Run(tc.name, func(t *testing.T) {
			doc := fmt.Sprintf(`
name: test
skills:
  - name: %q
    source: https://github.com/user/repo.git
    path: skills/thing
`, tc.value)

			_, err := ParseBytes([]byte(doc))
			if err == nil {
				t.Fatalf("ParseBytes() accepted unsafe name %q, want error", tc.value)
			}
		})
	}
}

func TestParseBytes_AcceptsSafeNames(t *testing.T) {
	safe := []string{
		"frontend-design",
		"skill_creator",
		"Skill123",
		"a",
		"my.skill",
	}

	for _, name := range safe {
		t.Run(name, func(t *testing.T) {
			doc := fmt.Sprintf(`
name: test
skills:
  - name: %q
    source: https://github.com/user/repo.git
    path: skills/thing
`, name)

			m, err := ParseBytes([]byte(doc))
			if err != nil {
				t.Fatalf("ParseBytes() rejected safe name %q: %v", name, err)
			}
			if m.Skills[0].Name != name {
				t.Errorf("Name = %q, want %q", m.Skills[0].Name, name)
			}
		})
	}
}

func TestParseBytes_RejectsWhitespacePaddedName(t *testing.T) {
	doc := `
name: test
skills:
  - name: "  padded  "
    source: https://github.com/user/repo.git
    path: skills/thing
`
	_, err := ParseBytes([]byte(doc))
	if err == nil {
		t.Fatal("ParseBytes() accepted a whitespace-padded name, want error")
	}
	if !strings.Contains(err.Error(), "whitespace") {
		t.Errorf("error = %q, want it to mention whitespace", err.Error())
	}
}
