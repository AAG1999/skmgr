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
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestGetAgent_Builtin(t *testing.T) {
	agents := DefaultAgents()

	// 1. Cursor
	cursor, ok := agents["cursor"]
	if !ok || cursor.Name != "cursor" {
		t.Error("TestGetAgent_Cursor failed")
	}
	if cursor.RuleStrategyForScope(ScopeProject) != RuleStrategySymlink {
		t.Error("Cursor should use symlink strategy")
	}

	// 2. Gemini
	gemini, ok := agents["gemini"]
	if !ok || gemini.Name != "gemini" {
		t.Error("TestGetAgent_Gemini failed")
	}
	if gemini.RuleStrategyForScope(ScopeProject) != RuleStrategySkip {
		t.Error("Gemini project scope should use skip strategy")
	}
	if gemini.RuleStrategyForScope(ScopeGlobal) != RuleStrategyMerge {
		t.Error("Gemini global scope should use merge strategy")
	}

	// 3. Claude Code
	claude, ok := agents["claude-code"]
	if !ok || claude.Name != "claude-code" {
		t.Error("TestGetAgent_ClaudeCode failed")
	}
	if claude.RuleStrategyForScope(ScopeProject) != RuleStrategyMerge {
		t.Error("Claude Code should use merge strategy")
	}

	// 4. Copilot
	copilot, ok := agents["copilot"]
	if !ok || copilot.Name != "copilot" {
		t.Error("TestGetAgent_Copilot failed")
	}
	if copilot.RuleStrategyForScope(ScopeProject) != RuleStrategyMerge {
		t.Error("Copilot should use merge strategy")
	}

	// 5. Unknown
	_, ok = agents["unknown-agent"]
	if ok {
		t.Error("TestGetAgent_Unknown failed: found unknown agent")
	}
}

func TestListAgents(t *testing.T) {
	expected := []string{"cursor", "gemini", "claude-code", "copilot"}
	actual := SupportedAgentNames()
	sort.Strings(expected)
	sort.Strings(actual)

	if !reflect.DeepEqual(expected, actual) {
		t.Errorf("SupportedAgentNames() = %v, want %v", actual, expected)
	}
}

func TestCursorSkillsDir_Project(t *testing.T) {
	agents := DefaultAgents()
	cursor := agents["cursor"]
	root := "/my/project"
	expected := filepath.Join(root, ".cursor", "skills")
	actual := cursor.SkillsDir(ScopeProject, root)
	if actual != expected {
		t.Errorf("Cursor project SkillsDir = %q, want %q", actual, expected)
	}
}

// Global scope must anchor on the user's home directory, ignoring whichever
// project root happens to be passed in.
func TestCursorSkillsDir_Global(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	agents := DefaultAgents()
	cursor := agents["cursor"]

	expected := filepath.Join(home, ".cursor", "skills")
	actual := cursor.SkillsDir(ScopeGlobal, "/my/project")
	if actual != expected {
		t.Errorf("Cursor global SkillsDir = %q, want %q", actual, expected)
	}
}

func TestGeminiSkillsDir_Project(t *testing.T) {
	agents := DefaultAgents()
	gemini := agents["gemini"]
	root := "/my/project"

	// Gemini reads from .agents/ natively at project scope
	expected := filepath.Join(root, ".agents", "skills")
	actual := gemini.SkillsDir(ScopeProject, root)
	if actual != expected {
		t.Errorf("Gemini project SkillsDir = %q, want %q", actual, expected)
	}
}

func TestGeminiSkillsDir_Global(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	agents := DefaultAgents()
	gemini := agents["gemini"]

	expected := filepath.Join(home, ".gemini", "config", "skills")
	actual := gemini.SkillsDir(ScopeGlobal, "/my/project")
	if actual != expected {
		t.Errorf("Gemini global SkillsDir = %q, want %q", actual, expected)
	}
}

// Regression guard: a global install writing under the project root would put
// user-wide skills inside whatever directory the command happened to run in.
func TestAgentDirs_GlobalScopeNeverUsesProjectRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	projectRoot := t.TempDir()

	for name, agent := range DefaultAgents() {
		t.Run(name, func(t *testing.T) {
			skillsDir := agent.SkillsDir(ScopeGlobal, projectRoot)
			if strings.HasPrefix(skillsDir, projectRoot) {
				t.Errorf("global SkillsDir %q is inside the project root %q", skillsDir, projectRoot)
			}
			if !strings.HasPrefix(skillsDir, home) {
				t.Errorf("global SkillsDir %q is not under the home directory %q", skillsDir, home)
			}

			rulesPath := agent.RulesPath(ScopeGlobal, projectRoot)
			if strings.HasPrefix(rulesPath, projectRoot) {
				t.Errorf("global RulesPath %q is inside the project root %q", rulesPath, projectRoot)
			}
		})
	}
}

func TestGeminiReadsFromAgents_Project(t *testing.T) {
	agents := DefaultAgents()
	gemini := agents["gemini"]
	if !gemini.ReadsFromAgents(ScopeProject) {
		t.Error("Gemini should natively read from .agents/ at project scope")
	}
}

func TestGeminiReadsFromAgents_Global(t *testing.T) {
	agents := DefaultAgents()
	gemini := agents["gemini"]
	if gemini.ReadsFromAgents(ScopeGlobal) {
		t.Error("Gemini does NOT read from ~/.agents/ natively at global scope (it reads from ~/.gemini/config/)")
	}
}

func TestCanonicalDirs(t *testing.T) {
	root := "/my/project"
	home, _ := os.UserHomeDir()

	projSkills := CanonicalSkillsDir(ScopeProject, root)
	if projSkills != filepath.Join(root, ".agents", "skills") {
		t.Errorf("Unexpected projSkills: %s", projSkills)
	}

	globSkills := CanonicalSkillsDir(ScopeGlobal, root)
	if globSkills != filepath.Join(home, ".agents", "skills") {
		t.Errorf("Unexpected globSkills: %s", globSkills)
	}

	projRules := CanonicalRulesDir(ScopeProject, root)
	if projRules != filepath.Join(root, ".agents", "rules") {
		t.Errorf("Unexpected projRules: %s", projRules)
	}
}

func TestCacheDir(t *testing.T) {
	cache := CacheDir()
	home, _ := os.UserHomeDir()
	if !strings.HasPrefix(cache, home) || !strings.HasSuffix(cache, filepath.Join(".skmgr", "cache")) {
		t.Errorf("Unexpected cache dir: %s", cache)
	}
}
