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

package types

import (
	"os"
	"path/filepath"

	"github.com/AAG1999/skmgr/internal/config"
)

// RuleStrategy defines how rules are installed for an agent.
type RuleStrategy string

const (
	// RuleStrategySymlink means rules are symlinked as directories
	// (e.g., Cursor's .cursor/rules/<name>/).
	RuleStrategySymlink RuleStrategy = "symlink"

	// RuleStrategyMerge means rule content is merged into a single file
	// using skmgr-managed delimiters (e.g., Claude's .claude/CLAUDE.md).
	RuleStrategyMerge RuleStrategy = "merge"

	// RuleStrategySkip means no action is needed because the agent
	// natively reads from .agents/ (e.g., Gemini at project scope).
	RuleStrategySkip RuleStrategy = "skip"
)

// AgentDef defines how a specific AI agent discovers skills and rules.
type AgentDef struct {
	// Name is the agent identifier used in skmgr.yml targets
	// (e.g., "cursor", "gemini", "claude-code", "copilot").
	Name string

	// SkillsDir returns the directory path where this agent looks for skills.
	// scope is "project" or "global", root is the project or home directory.
	SkillsDir func(scope string, root string) string

	// RulesPath returns the path where this agent looks for rules.
	// For symlink strategy, this is a directory.
	// For merge strategy, this is the single rules file path.
	RulesPath func(scope string, root string) string

	// RuleStrategy determines how rules are installed for this agent.
	// May differ by scope (e.g., Gemini uses "skip" at project scope,
	// "merge" at global scope).
	RuleStrategyForScope func(scope string) RuleStrategy

	// ReadsFromAgents returns true if this agent natively reads from
	// the .agents/ directory at the given scope, meaning no symlink
	// is needed for skills.
	ReadsFromAgents func(scope string) bool
}

// scopeRoot returns the filesystem root an agent's directories hang off for the
// given scope. Global installs must anchor on the user's home directory so they
// never write into whichever project happens to be the current working
// directory. If the home directory cannot be determined we fall back to the
// project root rather than failing the whole operation.
func scopeRoot(scope string, projectRoot string) string {
	if scope == ScopeGlobal {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	return projectRoot
}

// DefaultAgents returns the built-in agent definitions.
func DefaultAgents() map[string]AgentDef {
	return map[string]AgentDef{
		"cursor":      cursorAgent(),
		"gemini":      geminiAgent(),
		"claude-code": claudeCodeAgent(),
		"copilot":     copilotAgent(),
	}
}

// SupportedAgentNames returns the names of all built-in agents.
func SupportedAgentNames() []string {
	return []string{"cursor", "gemini", "claude-code", "copilot"}
}

func cursorAgent() AgentDef {
	return AgentDef{
		Name: "cursor",
		SkillsDir: func(scope string, root string) string {
			return filepath.Join(scopeRoot(scope, root), ".cursor", "skills")
		},
		RulesPath: func(scope string, root string) string {
			return filepath.Join(scopeRoot(scope, root), ".cursor", "rules")
		},
		RuleStrategyForScope: func(_ string) RuleStrategy {
			return RuleStrategySymlink
		},
		ReadsFromAgents: func(_ string) bool {
			return false
		},
	}
}

func geminiAgent() AgentDef {
	return AgentDef{
		Name: "gemini",
		SkillsDir: func(scope string, root string) string {
			if scope == ScopeGlobal {
				return filepath.Join(scopeRoot(scope, root), ".gemini", "config", "skills")
			}
			// Project scope: Gemini reads from .agents/skills/ natively.
			return filepath.Join(root, ".agents", "skills")
		},
		RulesPath: func(scope string, root string) string {
			if scope == ScopeGlobal {
				return filepath.Join(scopeRoot(scope, root), ".gemini", "config", "AGENTS.md")
			}
			return filepath.Join(root, ".agents", "AGENTS.md")
		},
		RuleStrategyForScope: func(scope string) RuleStrategy {
			if scope == ScopeGlobal {
				return RuleStrategyMerge
			}
			// Project scope: Gemini reads from .agents/ natively.
			return RuleStrategySkip
		},
		ReadsFromAgents: func(scope string) bool {
			return scope == ScopeProject
		},
	}
}

func claudeCodeAgent() AgentDef {
	return AgentDef{
		Name: "claude-code",
		SkillsDir: func(scope string, root string) string {
			return filepath.Join(scopeRoot(scope, root), ".claude", "skills")
		},
		RulesPath: func(scope string, root string) string {
			return filepath.Join(scopeRoot(scope, root), ".claude", "CLAUDE.md")
		},
		RuleStrategyForScope: func(_ string) RuleStrategy {
			return RuleStrategyMerge
		},
		ReadsFromAgents: func(_ string) bool {
			return false
		},
	}
}

func copilotAgent() AgentDef {
	return AgentDef{
		Name: "copilot",
		SkillsDir: func(scope string, root string) string {
			return filepath.Join(scopeRoot(scope, root), ".github", "skills")
		},
		RulesPath: func(scope string, root string) string {
			return filepath.Join(scopeRoot(scope, root), ".github", "copilot-instructions.md")
		},
		RuleStrategyForScope: func(_ string) RuleStrategy {
			return RuleStrategyMerge
		},
		ReadsFromAgents: func(_ string) bool {
			return false
		},
	}
}

// CanonicalDir returns the canonical .agents/ directory path where
// skills and rules are stored before symlinking.
func CanonicalDir(scope string, projectRoot string) string {
	if scope == ScopeGlobal {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".agents")
	}
	return filepath.Join(projectRoot, ".agents")
}

// CanonicalSkillsDir returns the canonical path for skills storage.
func CanonicalSkillsDir(scope string, projectRoot string) string {
	return filepath.Join(CanonicalDir(scope, projectRoot), "skills")
}

// CanonicalRulesDir returns the canonical path for rules storage.
func CanonicalRulesDir(scope string, projectRoot string) string {
	return filepath.Join(CanonicalDir(scope, projectRoot), "rules")
}

// SkmgrHome returns the directory holding skmgr's own state: the git cache and
// the global install ledger. It honors SKMGR_HOME and the config file.
func SkmgrHome() string {
	return config.Current().Home
}

// CacheDir returns the global cache directory for cloned git repos. It honors
// SKMGR_CACHE_DIR and the config file, so a CI job can point it at a directory
// it already caches without relocating the rest of skmgr's state.
func CacheDir() string {
	return config.Current().CacheDir
}
