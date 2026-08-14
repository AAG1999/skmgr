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

package linker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
)

// LinkIssue represents a detected problem during Verification.
type LinkIssue struct {
	Agent   string
	Path    string
	Message string
}

// Linker orchestrates symlink creation and rule merging.
type Linker struct {
	Agents map[string]types.AgentDef
}

// NewLinker creates a new Linker instance using the default agents.
func NewLinker() *Linker {
	return &Linker{
		Agents: types.DefaultAgents(),
	}
}

// LinkSkill installs a skill for a specific agent.
func (l *Linker) LinkSkill(agentName string, skillName string, scope string, projectRoot string) error {
	agent, ok := l.Agents[agentName]
	if !ok {
		return fmt.Errorf("unknown agent %q", agentName)
	}

	// If the agent natively reads from the canonical .agents/ directory at this scope,
	// we don't need to create a symlink.
	if agent.ReadsFromAgents(scope) {
		return nil
	}

	canonicalSrc := filepath.Join(types.CanonicalSkillsDir(scope, projectRoot), skillName)
	agentDir := agent.SkillsDir(scope, projectRoot)
	linkPath := filepath.Join(agentDir, skillName)

	if err := createSymlink(canonicalSrc, linkPath); err != nil {
		return fmt.Errorf("linking skill %q for %q: %w", skillName, agentName, err)
	}

	// Update .gitignore if at project scope
	if scope == types.ScopeProject {
		relLink, err := filepath.Rel(projectRoot, linkPath)
		if err == nil {
			if err := ensureGitignoreEntry(projectRoot, relLink); err != nil {
				return fmt.Errorf("updating gitignore for skill %q: %w", skillName, err)
			}
		}
	}

	return nil
}

// LinkRule installs a rule for a specific agent based on its RuleStrategy.
func (l *Linker) LinkRule(agentName string, ruleName string, scope string, projectRoot string) error {
	agent, ok := l.Agents[agentName]
	if !ok {
		return fmt.Errorf("unknown agent %q", agentName)
	}

	strategy := agent.RuleStrategyForScope(scope)
	canonicalSrc := filepath.Join(types.CanonicalRulesDir(scope, projectRoot), ruleName)

	switch strategy {
	case types.RuleStrategySkip:
		// Do nothing
		return nil

	case types.RuleStrategySymlink:
		agentDir := agent.RulesPath(scope, projectRoot)
		linkPath := filepath.Join(agentDir, ruleName)
		if err := createSymlink(canonicalSrc, linkPath); err != nil {
			return fmt.Errorf("symlinking rule %q for %q: %w", ruleName, agentName, err)
		}
		if scope == types.ScopeProject {
			if relLink, err := filepath.Rel(projectRoot, linkPath); err == nil {
				if err := ensureGitignoreEntry(projectRoot, relLink); err != nil {
					return fmt.Errorf("updating gitignore for rule %q: %w", ruleName, err)
				}
			}
		}

	case types.RuleStrategyMerge:
		targetFile := agent.RulesPath(scope, projectRoot)

		data, err := readRuleContent(canonicalSrc, ruleName)
		if err != nil {
			return err
		}

		if err := mergeSection(targetFile, ruleName, data); err != nil {
			return fmt.Errorf("merging rule %q for %q: %w", ruleName, agentName, err)
		}
	}

	return nil
}

// ruleContentFilenames are the files that can hold a rule's body, in the order
// they are tried. AGENTS.md comes first because that is what marks a directory
// as a rule in the first place; SKILL.md is accepted for skills converted to
// rules and for sources that only ship that filename.
var ruleContentFilenames = []string{"AGENTS.md", "SKILL.md", "rule.md", "README.md"}

// readRuleContent locates the body of a rule. A rule is normally a directory,
// but a bare file is accepted too.
func readRuleContent(canonicalSrc string, ruleName string) (string, error) {
	info, err := os.Stat(canonicalSrc)
	if err != nil {
		return "", fmt.Errorf("reading canonical rule %q: %w", ruleName, err)
	}

	if !info.IsDir() {
		data, err := os.ReadFile(canonicalSrc)
		if err != nil {
			return "", fmt.Errorf("reading canonical rule %q: %w", ruleName, err)
		}
		return string(data), nil
	}

	for _, candidate := range ruleContentFilenames {
		data, err := os.ReadFile(filepath.Join(canonicalSrc, candidate))
		if err == nil {
			return string(data), nil
		}
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("reading canonical rule %q: %w", ruleName, err)
		}
	}

	return "", fmt.Errorf(
		"rule %q has no content file in %s (looked for %s)",
		ruleName, canonicalSrc, strings.Join(ruleContentFilenames, ", "))
}

// UnlinkSkill removes a skill symlink for a specific agent.
func (l *Linker) UnlinkSkill(agentName string, skillName string, scope string, projectRoot string) error {
	agent, ok := l.Agents[agentName]
	if !ok {
		return fmt.Errorf("unknown agent %q", agentName)
	}

	if agent.ReadsFromAgents(scope) {
		return nil
	}

	linkPath := filepath.Join(agent.SkillsDir(scope, projectRoot), skillName)
	if err := removeSymlink(linkPath); err != nil {
		return fmt.Errorf("unlinking skill %q for %q: %w", skillName, agentName, err)
	}

	if scope == types.ScopeProject {
		if relLink, err := filepath.Rel(projectRoot, linkPath); err == nil {
			if err := removeGitignoreEntry(projectRoot, relLink); err != nil {
				return fmt.Errorf("updating gitignore for unlinked skill %q: %w", skillName, err)
			}
		}
	}

	return nil
}

// UnlinkRule removes a rule for a specific agent.
func (l *Linker) UnlinkRule(agentName string, ruleName string, scope string, projectRoot string) error {
	agent, ok := l.Agents[agentName]
	if !ok {
		return fmt.Errorf("unknown agent %q", agentName)
	}

	strategy := agent.RuleStrategyForScope(scope)

	switch strategy {
	case types.RuleStrategySkip:
		return nil

	case types.RuleStrategySymlink:
		linkPath := filepath.Join(agent.RulesPath(scope, projectRoot), ruleName)
		if err := removeSymlink(linkPath); err != nil {
			return fmt.Errorf("unlinking rule %q for %q: %w", ruleName, agentName, err)
		}
		if scope == types.ScopeProject {
			if relLink, err := filepath.Rel(projectRoot, linkPath); err == nil {
				if err := removeGitignoreEntry(projectRoot, relLink); err != nil {
					return fmt.Errorf("updating gitignore for unlinked rule %q: %w", ruleName, err)
				}
			}
		}

	case types.RuleStrategyMerge:
		targetFile := agent.RulesPath(scope, projectRoot)
		if err := removeSection(targetFile, ruleName); err != nil {
			return fmt.Errorf("removing merged rule %q for %q: %w", ruleName, agentName, err)
		}
	}

	return nil
}

// Verify checks all requested links for a given manifest.
func (l *Linker) Verify(manifest *types.Manifest, scope string, projectRoot string) []LinkIssue {
	var issues []LinkIssue

	for _, skill := range manifest.Skills {
		// Only check skills targeting the specified scope
		if skill.EffectiveScope() != scope {
			continue
		}

		targets := skill.EffectiveTargets(manifest.Targets)
		for _, target := range targets {
			agent, ok := l.Agents[target]
			if !ok {
				issues = append(issues, LinkIssue{Agent: target, Path: "", Message: "Unknown agent"})
				continue
			}

			if skill.EffectiveType() == types.TypeSkill {
				if agent.ReadsFromAgents(scope) {
					continue
				}
				linkPath := filepath.Join(agent.SkillsDir(scope, projectRoot), skill.Name)
				if !isSymlink(linkPath) {
					issues = append(issues, LinkIssue{Agent: target, Path: linkPath, Message: "Symlink missing"})
					continue
				}
				if !linkTargetExists(linkPath) {
					issues = append(issues, LinkIssue{Agent: target, Path: linkPath, Message: "Symlink points to non-existent target"})
				}
			} else if agent.RuleStrategyForScope(scope) == types.RuleStrategySymlink {
				// Rule verification could check for symlink or delimiters
				// For now, only checking symlink rules for simplicity
				linkPath := filepath.Join(agent.RulesPath(scope, projectRoot), skill.Name)
				if !isSymlink(linkPath) {
					issues = append(issues, LinkIssue{Agent: target, Path: linkPath, Message: "Symlink missing"})
				}
			}
		}
	}

	return issues
}

// CleanBrokenLinks sweeps the known agent directories for the given scope and removes any broken symlinks.
func (l *Linker) CleanBrokenLinks(scope string, projectRoot string) error {
	for _, agent := range l.Agents {
		// Clean skills dir
		if !agent.ReadsFromAgents(scope) {
			skillsDir := agent.SkillsDir(scope, projectRoot)
			if err := cleanBrokenSymlinksInDir(skillsDir, projectRoot); err != nil {
				return fmt.Errorf("cleaning broken skill links for agent %q: %w", agent.Name, err)
			}
		}

		// Clean rules dir if symlinked
		if agent.RuleStrategyForScope(scope) == types.RuleStrategySymlink {
			rulesDir := agent.RulesPath(scope, projectRoot)
			if err := cleanBrokenSymlinksInDir(rulesDir, projectRoot); err != nil {
				return fmt.Errorf("cleaning broken rule links for agent %q: %w", agent.Name, err)
			}
		}
	}
	return nil
}

// StripManagedRules removes every skmgr-managed block from the rule files of
// agents that use the merge strategy. A merged rule leaves no symlink, so
// clearing an install otherwise leaves blocks referring to skills that are gone.
func (l *Linker) StripManagedRules(scope string, projectRoot string) error {
	for _, agent := range l.Agents {
		if agent.RuleStrategyForScope(scope) != types.RuleStrategyMerge {
			continue
		}
		targetFile := agent.RulesPath(scope, projectRoot)
		if err := removeAllSections(targetFile); err != nil {
			return fmt.Errorf("stripping managed rules for agent %q: %w", agent.Name, err)
		}
	}
	return nil
}

func cleanBrokenSymlinksInDir(dir string, projectRoot string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		info, err := os.Lstat(path)
		if err != nil {
			continue
		}

		// A relative link has to be resolved against its own directory. Judging
		// it from the working directory would make intact links look broken
		// whenever skmgr runs from a subdirectory, and this loop deletes what
		// it considers broken.
		if info.Mode()&os.ModeSymlink != 0 && !linkTargetExists(path) {
			if err := os.Remove(path); err != nil {
				return fmt.Errorf("removing broken symlink %q: %w", path, err)
			}
			if relLink, err := filepath.Rel(projectRoot, path); err == nil {
				if err := removeGitignoreEntry(projectRoot, relLink); err != nil {
					return fmt.Errorf("removing gitignore entry for %q: %w", relLink, err)
				}
			}
		}
	}
	return nil
}
