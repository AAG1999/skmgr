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

	yaml "gopkg.in/yaml.v3"
)

// locations records the line each part of a manifest was written on.
//
// The struct the YAML decodes into has no memory of the file it came from, so
// a validation error can only say "skills[3]" and leave the user counting
// entries. Decoding a second time into a node tree is cheap and lets every
// message point at a line the editor can jump to.
type locations struct {
	// top maps a top-level key to its line.
	top map[string]int

	// skills maps a skill's index to the line its entry starts on.
	skills []int

	// skillKeys maps a skill's index to the line of each of its keys.
	skillKeys []map[string]int
}

// newLocations reads the line numbers out of a manifest.
//
// A document that will not parse as a node tree yields empty locations rather
// than an error: the decoder that runs alongside this reports the syntax
// problem, and it does so with a line of its own.
func newLocations(data []byte) *locations {
	loc := &locations{top: map[string]int{}}

	var doc yaml.Node
	if err := yaml.NewDecoder(bytes.NewReader(data)).Decode(&doc); err != nil {
		return loc
	}

	root := documentRoot(&doc)
	if root == nil || root.Kind != yaml.MappingNode {
		return loc
	}

	for i := 0; i+1 < len(root.Content); i += 2 {
		key, value := root.Content[i], root.Content[i+1]
		loc.top[key.Value] = key.Line

		if key.Value == "skills" && value.Kind == yaml.SequenceNode {
			loc.readSkills(value)
		}
	}

	return loc
}

// readSkills records the line of every skill entry and of its keys.
func (l *locations) readSkills(sequence *yaml.Node) {
	for _, entry := range sequence.Content {
		l.skills = append(l.skills, entry.Line)

		keys := map[string]int{}
		if entry.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(entry.Content); i += 2 {
				keys[entry.Content[i].Value] = entry.Content[i].Line
			}
		}
		l.skillKeys = append(l.skillKeys, keys)
	}
}

// documentRoot unwraps a document node.
func documentRoot(node *yaml.Node) *yaml.Node {
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return nil
		}
		return node.Content[0]
	}
	return node
}

// topLine is the line a top-level key is on, or 0 if it is absent.
func (l *locations) topLine(key string) int {
	if l == nil {
		return 0
	}
	return l.top[key]
}

// skillLine is the line a skill entry starts on, or 0 if it is unknown.
func (l *locations) skillLine(index int) int {
	if l == nil || index < 0 || index >= len(l.skills) {
		return 0
	}
	return l.skills[index]
}

// skillKeyLine is the line one of a skill's keys is on. It falls back to the
// line of the entry itself, which is the right place to point at when the
// problem is a key the user did not write.
func (l *locations) skillKeyLine(index int, key string) int {
	if l == nil || index < 0 || index >= len(l.skillKeys) {
		return 0
	}
	if line, ok := l.skillKeys[index][key]; ok {
		return line
	}
	return l.skillLine(index)
}

// at prefixes a message with a line number when one is known.
//
// The `line N:` form is what compilers and linters emit, so editors already
// know how to turn it into a link.
func at(line int, format string, args ...any) error {
	message := fmt.Sprintf(format, args...)
	if line <= 0 {
		return fmt.Errorf("manifest validation: %s", message)
	}
	return fmt.Errorf("manifest validation: line %d: %s", line, message)
}
