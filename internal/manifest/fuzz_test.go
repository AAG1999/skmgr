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

// seeds are manifests that have caused trouble, or are close to something that
// could. A fuzzer left to invent YAML from nothing spends its time on syntax
// errors; starting from real shapes puts it near the interesting cases.
var seeds = []string{
	"",
	"\n",
	"---\n",
	"version: \"1\"\nname: demo\n",
	"version: \"1\"\nname: demo\nskills: []\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./local\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./local\n    path: skills/one\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./local\n    path: skills/*\n",
	"version: \"1\"\nname: demo\ntargets:\n  - cursor\n  - gemini\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./a\n    name: ../escape\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./a\n    path: ../../etc\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./a\n    name: CON\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./a\n    name: Greeter\n  - source: ./b\n    name: greeter\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: git@github.com:o/r.git\n    ref: ^1.2.0\n",
	"version: \"1\"\nname: demo\nunknown_key: 1\n",
	"name: [not, a, string]\n",
	"version: \"1\"\nname: demo\nskills: not-a-list\n",
	"a: &x [*x]\n",
	"version: \"1\"\nname: \"\\u0000\"\n",
	"version: \"1\"\nname: demo\nskills:\n  - source: ./a\n    name: \"a\\u0000b\"\n",
}

// FuzzParseBytes checks that no input makes the parser panic, and that anything
// it accepts is safe for the rest of skmgr to act on.
//
// The parser is the only thing between a file someone downloaded and code that
// joins names onto filesystem paths, so "it returned an error" is not the
// interesting property. What matters is that a manifest which parses cleanly
// cannot then escape the install tree.
func FuzzParseBytes(f *testing.F) {
	for _, seed := range seeds {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		m, err := ParseBytes(data)
		if err != nil {
			if m != nil {
				t.Fatal("a failed parse must not return a manifest")
			}
			return
		}

		if m == nil {
			t.Fatal("a successful parse must return a manifest")
		}

		if strings.TrimSpace(m.Name) == "" {
			t.Fatalf("accepted a manifest with no project name: %q", data)
		}

		for i, skill := range m.Skills {
			assertUsableSkill(t, i, skill, data)
		}
	})
}

// assertUsableSkill states what every accepted skill must satisfy.
func assertUsableSkill(t *testing.T, index int, skill types.SkillDependency, data []byte) {
	t.Helper()

	if strings.TrimSpace(skill.Source) == "" {
		t.Fatalf("skills[%d] was accepted with no source: %q", index, data)
	}

	// A wildcard entry is named per match at resolve time, so it is the one
	// case where a blank name is expected.
	if skill.Name == "" {
		if !IsWildcard(skill.Path) {
			t.Fatalf("skills[%d] was accepted with no name: %q", index, data)
		}
		return
	}

	// The name becomes a directory under .agents/ and under every agent's
	// directory. Anything that is not a single, plain path component is a way
	// out of the install tree.
	if err := types.ValidateName(skill.Name); err != nil {
		t.Fatalf("skills[%d] has an unusable name %q: %v (input %q)", index, skill.Name, err, data)
	}

	if strings.ContainsAny(skill.Name, `/\`) {
		t.Fatalf("skills[%d] name %q contains a path separator (input %q)", index, skill.Name, data)
	}
	if skill.Name != filepath.Base(skill.Name) {
		t.Fatalf("skills[%d] name %q is not a single path component (input %q)", index, skill.Name, data)
	}
	if skill.Name == "." || skill.Name == ".." {
		t.Fatalf("skills[%d] name %q is a directory traversal (input %q)", index, skill.Name, data)
	}
	if strings.ContainsRune(skill.Name, 0) {
		t.Fatalf("skills[%d] name contains a NUL byte (input %q)", index, data)
	}

	if skill.Type != "" && !contains(types.ValidTypes(), skill.Type) {
		t.Fatalf("skills[%d] has invalid type %q (input %q)", index, skill.Type, data)
	}
	if skill.Scope != "" && !contains(types.ValidScopes(), skill.Scope) {
		t.Fatalf("skills[%d] has invalid scope %q (input %q)", index, skill.Scope, data)
	}

	for _, target := range skill.Targets {
		if !contains(types.SupportedAgentNames(), target) {
			t.Fatalf("skills[%d] has unknown target %q (input %q)", index, target, data)
		}
	}
}

// FuzzDeriveName checks the name derivation helper directly, since it turns
// arbitrary source and path strings into a directory name.
func FuzzDeriveName(f *testing.F) {
	pairs := [][2]string{
		{"https://github.com/owner/repo.git", "skills/one"},
		{"https://github.com/owner/repo.git", ""},
		{"git@github.com:owner/repo.git", ""},
		{"./local", "../escape"},
		{"./local", "/"},
		{"./local", "//"},
		{"", ""},
		{"file:///", ""},
		{"git::../mirror", "skills/x"},
		{"repo.git/", ""},
	}
	for _, pair := range pairs {
		f.Add(pair[0], pair[1])
	}

	f.Fuzz(func(t *testing.T, source string, path string) {
		name, err := DeriveName(source, path)
		if err != nil {
			return
		}

		// A derived name goes through the same validation as a written one, so
		// anything derivable must also be usable. Returning a name the parser
		// then rejects would be a confusing failure with no user-visible cause.
		if err := types.ValidateName(name); err != nil {
			t.Fatalf("DeriveName(%q, %q) = %q, which is not a usable name: %v", source, path, name, err)
		}
	})
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}
