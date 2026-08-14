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
	"bytes"
	"fmt"

	"github.com/AAG1999/skmgr/internal/types"
	"github.com/AAG1999/skmgr/internal/util/atomicfile"
	yaml "gopkg.in/yaml.v3"
)

// Write saves the manifest back to the given file path.
// It uses yaml.v3 to encode the struct to YAML.
// Note: yaml.v3 will drop comments when marshalling from a struct.
func Write(path string, m *types.Manifest) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)

	if err := enc.Encode(forWriting(m)); err != nil {
		return fmt.Errorf("encoding manifest to YAML: %w", err)
	}

	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing YAML encoder: %w", err)
	}

	// Write the file, creating it if it doesn't exist, truncating if it does.
	if err := atomicfile.Write(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing manifest file: %w", err)
	}

	return nil
}

// forWriting returns a copy with everything skmgr filled in on the user's
// behalf stripped back out. Parsing populates derived names and normalizes
// defaults, and writing that back would turn every rewrite into a diff full of
// values the user never typed.
func forWriting(m *types.Manifest) *types.Manifest {
	out := *m
	out.Skills = make([]types.SkillDependency, len(m.Skills))
	copy(out.Skills, m.Skills)

	for i := range out.Skills {
		skill := &out.Skills[i]

		if skill.NameDerived {
			skill.Name = ""
		}
		if skill.Type == types.TypeSkill {
			skill.Type = ""
		}
		if skill.Scope == types.ScopeProject {
			skill.Scope = ""
		}
	}

	return &out
}
