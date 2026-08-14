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
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
	"github.com/xeipuuv/gojsonschema"
	yaml "gopkg.in/yaml.v3"
)

// schemaPath is the schema shipped for editor autocomplete. It has to agree
// with the parser: an editor that accepts a manifest skmgr then rejects is
// worse than no schema at all.
const schemaPath = "../../schemas/skmgr.schema.json"

// loadSchema compiles the shipped schema.
//
// The bytes are loaded directly rather than through a file:// reference: an
// absolute Windows path starts with a drive letter, so "file://" + path is not
// a parseable URL and the loader rejects it before reading anything.
func loadSchema(t *testing.T) *gojsonschema.Schema {
	t.Helper()

	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("the schema should ship with the repository: %v", err)
	}

	schema, err := gojsonschema.NewSchema(gojsonschema.NewBytesLoader(data))
	if err != nil {
		t.Fatalf("the shipped schema is not valid JSON Schema: %v", err)
	}
	return schema
}

// validateAgainstSchema converts a YAML manifest to JSON and validates it.
func validateAgainstSchema(t *testing.T, schema *gojsonschema.Schema, manifestYAML string) []string {
	t.Helper()

	var raw any
	if err := yaml.Unmarshal([]byte(manifestYAML), &raw); err != nil {
		t.Fatalf("the test manifest is not valid YAML: %v", err)
	}

	encoded, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("converting the manifest to JSON: %v", err)
	}

	result, err := schema.Validate(gojsonschema.NewBytesLoader(encoded))
	if err != nil {
		t.Fatalf("validating: %v", err)
	}

	var problems []string
	for _, desc := range result.Errors() {
		problems = append(problems, desc.String())
	}
	return problems
}

func TestSchema_IsValidJSON(t *testing.T) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("the schema should ship with the repository: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("the schema is not valid JSON: %v", err)
	}

	if parsed["$id"] == "" || parsed["$id"] == nil {
		t.Error("the schema needs an $id so editors can reference it by URL")
	}
}

// The two validators have to agree, so every manifest the schema accepts is
// checked against the parser as well.
func TestSchema_AcceptsValidManifests(t *testing.T) {
	schema := loadSchema(t)

	cases := map[string]string{
		"minimal": `
name: demo
skills: []
`,
		"every field": `
name: demo
version: "1"
targets:
  - cursor
  - claude-code
skills:
  - name: greeter
    source: https://github.com/org/skills.git
    path: skills/greeter
    ref: ^1.2.0
    type: skill
    scope: project
    targets:
      - cursor
`,
		"wildcard path": `
name: demo
skills:
  - source: https://github.com/org/skills.git
    path: "skills/*"
    ref: main
`,
		"local source": `
name: demo
skills:
  - source: file:///home/me/skills
    path: skills/greeter
`,
		"rule at global scope": `
name: demo
targets:
  - claude-code
skills:
  - source: https://github.com/org/rules.git
    path: rules/style
    type: rule
    scope: global
`,
	}

	for name, manifestYAML := range cases {
		t.Run(name, func(t *testing.T) {
			if problems := validateAgainstSchema(t, schema, manifestYAML); len(problems) > 0 {
				t.Errorf("the schema rejected a valid manifest: %s", strings.Join(problems, "; "))
			}

			if _, err := ParseBytes([]byte(manifestYAML)); err != nil {
				t.Errorf("the parser rejected a manifest the schema accepts: %v", err)
			}
		})
	}
}

func TestSchema_RejectsInvalidManifests(t *testing.T) {
	schema := loadSchema(t)

	cases := map[string]struct {
		manifest string
		mentions string
	}{
		"missing name": {
			manifest: "skills: []\n",
			mentions: "name",
		},
		"skill without a source": {
			manifest: "name: demo\nskills:\n  - path: skills/greeter\n",
			mentions: "source",
		},
		"unknown agent": {
			manifest: "name: demo\ntargets:\n  - curser\n",
			mentions: "targets",
		},
		"invalid type": {
			manifest: "name: demo\nskills:\n  - source: x\n    type: prompt\n",
			mentions: "type",
		},
		"invalid scope": {
			manifest: "name: demo\nskills:\n  - source: x\n    scope: machine\n",
			mentions: "scope",
		},
		"misspelled key": {
			manifest: "name: demo\nskills:\n  - source: x\n    pth: skills/greeter\n",
			mentions: "pth",
		},
		"name with a separator": {
			manifest: "name: demo\nskills:\n  - source: x\n    name: nested/greeter\n",
			mentions: "name",
		},
		"duplicate targets": {
			manifest: "name: demo\ntargets:\n  - cursor\n  - cursor\n",
			mentions: "targets",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			problems := validateAgainstSchema(t, schema, tc.manifest)
			if len(problems) == 0 {
				t.Fatalf("the schema should have rejected this manifest")
			}

			joined := strings.Join(problems, "; ")
			if !strings.Contains(joined, tc.mentions) {
				t.Errorf("the complaint should mention %q, got: %s", tc.mentions, joined)
			}
		})
	}
}

// Anything the schema rejects, the parser must reject too, or an editor would
// flag a manifest that installs perfectly well.
func TestSchema_ParserAgreesOnRejections(t *testing.T) {
	rejected := []string{
		"skills: []\n",
		"name: demo\nskills:\n  - path: skills/greeter\n",
		"name: demo\ntargets:\n  - curser\n",
		"name: demo\nskills:\n  - source: x\n    type: prompt\n",
		"name: demo\nskills:\n  - source: x\n    scope: machine\n",
		"name: demo\nskills:\n  - source: x\n    pth: skills/greeter\n",
	}

	for _, manifestYAML := range rejected {
		if _, err := ParseBytes([]byte(manifestYAML)); err == nil {
			t.Errorf("the parser accepted a manifest the schema rejects:\n%s", manifestYAML)
		}
	}
}

// The agent list appears in both the schema and the code, and a new agent
// added to one and not the other would leave the schema quietly wrong.
func TestSchema_AgentListMatchesTheCode(t *testing.T) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatal(err)
	}

	var parsed struct {
		Definitions struct {
			Agent struct {
				Enum []string `json:"enum"`
			} `json:"agent"`
		} `json:"definitions"`
	}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}

	inSchema := strings.Join(parsed.Definitions.Agent.Enum, ",")
	inCode := strings.Join(types.SupportedAgentNames(), ",")

	if inSchema != inCode {
		t.Errorf("the schema lists agents %q but the code supports %q", inSchema, inCode)
	}
}
