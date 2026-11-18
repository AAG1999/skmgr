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

// Package workflow generates the chat skills and slash commands skmgr init
// writes into each agent directory.
package workflow

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
	"github.com/AAG1999/skmgr/internal/util/atomicfile"
)

// TemplateRevision changes only when generated file content changes, so an
// unrelated skmgr release does not rewrite files a project has committed.
const TemplateRevision = 2

//go:embed templates/*.md
var templateFS embed.FS

// Action is one workflow the user invokes from chat.
type Action struct {
	ID          string
	SkillName   string
	Description string
	Tools       []string
}

// Actions is the fixed set init generates, in getting-started order.
func Actions() []Action {
	return []Action{
		{ID: "find", SkillName: "skmgr-find-skill", Description: "Find a skill for a use case. Searches the web, then skmgr discover inspects the repository you approve. Does not install.", Tools: []string{"discover", "list"}},
		{ID: "install", SkillName: "skmgr-install-skill", Description: "Add a skill to skmgr.yml and install it after you confirm the plan.", Tools: []string{"init", "add"}},
		{ID: "update", SkillName: "skmgr-update-skill", Description: "Re-resolve skill refs and rewrite skmgr.lock, for every skill or only the names you give.", Tools: []string{"update", "outdated"}},
		{ID: "remove", SkillName: "skmgr-remove-skill", Description: "Remove a skill from skmgr.yml and from the agents after you confirm.", Tools: []string{"remove", "list"}},
	}
}

// TemplateSources returns the embedded template bodies, keyed by file name.
func TemplateSources() map[string]string {
	out := map[string]string{}
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return out
	}
	for _, entry := range entries {
		data, err := templateFS.ReadFile("templates/" + entry.Name())
		if err != nil {
			continue
		}
		out[entry.Name()] = lf(string(data))
	}
	return out
}

func body(action Action) (string, error) {
	preamble, err := templateFS.ReadFile("templates/preamble.md")
	if err != nil {
		return "", err
	}
	specific, err := templateFS.ReadFile("templates/" + action.ID + ".md")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(lf(string(preamble))) + "\n\n" + strings.TrimSpace(lf(string(specific))) + "\n", nil
}

// lf keeps generated text stable on Windows, where a checkout may have
// rewritten the embedded templates with CRLF.
func lf(text string) string {
	return strings.ReplaceAll(text, "\r\n", "\n")
}

func toolsList(tools []string) string {
	parts := make([]string, len(tools))
	for i, tool := range tools {
		parts[i] = "Bash(skmgr " + tool + ":*)"
	}
	return strings.Join(parts, ", ")
}

// File is one generated file.
type File struct {
	Rel     string
	Content string
	Kind    string // skill or command
}

// Render returns the skill and command files for one agent.
func Render(agent types.AgentDef, projectRoot string) ([]File, error) {
	var files []File
	for _, action := range Actions() {
		text, err := body(action)
		if err != nil {
			return nil, err
		}
		skillAbs := filepath.Join(agent.SkillsDir(types.ScopeProject, projectRoot), action.SkillName, "SKILL.md")
		skillRel, err := filepath.Rel(projectRoot, skillAbs)
		if err != nil {
			return nil, err
		}
		files = append(files, File{
			Rel:     filepath.ToSlash(skillRel),
			Content: stamp(skillFrontmatter(action) + text),
			Kind:    "skill",
		})
		if agent.CommandFile == nil {
			continue
		}
		command := commandFrontmatter(agent, action) + text
		if strings.HasSuffix(agent.CommandFile(action.ID), ".toml") {
			command += "\"\"\"\n"
		}
		files = append(files, File{
			Rel:     filepath.ToSlash(agent.CommandFile(action.ID)),
			Content: stamp(command),
			Kind:    "command",
		})
	}
	return files, nil
}

func skillFrontmatter(action Action) string {
	return fmt.Sprintf(`---
name: %s
description: %s
allowed-tools: %s
compatibility: Requires the skmgr CLI.
metadata:
  author: skmgr
  version: "1"
  generated: skmgr
  revision: %d
  hash: ""
---

`, action.SkillName, yamlEscape(action.Description), toolsList(action.Tools), TemplateRevision)
}

func commandFrontmatter(agent types.AgentDef, action Action) string {
	rel := agent.CommandFile(action.ID)
	switch {
	case strings.HasSuffix(rel, ".toml"):
		return fmt.Sprintf(`# skmgr:generated
# revision: %d
# hash: ""
description = "%s"

prompt = """
`, TemplateRevision, tomlEscape(action.Description))
	default:
		name := action.ID
		if agent.CommandInvocation != nil {
			name = agent.CommandInvocation(action.ID)
		}
		return fmt.Sprintf(`---
name: %s
description: %s
metadata:
  author: skmgr
  generated: skmgr
  revision: %d
  hash: ""
---

`, name, yamlEscape(action.Description), TemplateRevision)
	}
}

func yamlEscape(value string) string {
	return strings.ReplaceAll(value, "\"", "'")
}

func tomlEscape(value string) string {
	return strings.ReplaceAll(value, `"`, `\"`)
}

// FileAction is what init would do to one generated file.
type FileAction string

const (
	ActionAdd    FileAction = "add"
	ActionChange FileAction = "change"
	ActionDelete FileAction = "delete"
	ActionSkip   FileAction = "skip"
)

// Change is one planned write.
type Change struct {
	Path   string     `json:"path"`
	Action FileAction `json:"action"`
	Reason string     `json:"reason,omitempty"`
}

// Plan compares the files selected agents should have with what is on disk.
// Agents that are no longer selected lose unedited generated files.
func Plan(projectRoot string, targets []string, force bool) ([]Change, error) {
	selected := map[string]bool{}
	for _, target := range targets {
		selected[target] = true
	}
	agents := types.DefaultAgents()
	var changes []Change
	for _, name := range types.SupportedAgentNames() {
		agent := agents[name]
		files, err := Render(agent, projectRoot)
		if err != nil {
			return nil, err
		}
		if selected[name] {
			for _, file := range files {
				changes = append(changes, classify(projectRoot, file.Rel, file.Content, force))
			}
			continue
		}
		for _, file := range files {
			change := classify(projectRoot, file.Rel, file.Content, force)
			switch change.Action {
			case ActionAdd:
				continue
			case ActionSkip:
				changes = append(changes, change)
			default:
				changes = append(changes, Change{Path: file.Rel, Action: ActionDelete})
			}
		}
	}
	pruned := changes[:0]
	for _, change := range changes {
		if change.Action == "" {
			continue
		}
		pruned = append(pruned, change)
	}
	if pruned == nil {
		pruned = []Change{}
	}
	return pruned, nil
}

func classify(root, rel, next string, force bool) Change {
	abs := filepath.Join(root, filepath.FromSlash(rel))
	current, err := os.ReadFile(abs)
	if err != nil {
		return Change{Path: rel, Action: ActionAdd}
	}
	text := string(current)
	if !isGenerated(text) {
		return Change{Path: rel, Action: ActionSkip, Reason: "unmarked"}
	}
	if !hashMatches(text) && !force {
		return Change{Path: rel, Action: ActionSkip, Reason: "edited"}
	}
	if text == next {
		return Change{}
	}
	return Change{Path: rel, Action: ActionChange}
}

// Apply writes the add and change entries and deletes the delete entries.
// Skip entries are left untouched. Empty directories left behind are removed
// only when they are the generated skill or command directory.
func Apply(projectRoot string, changes []Change, files []File) error {
	byRel := map[string]string{}
	for _, file := range files {
		byRel[file.Rel] = file.Content
	}
	for _, change := range changes {
		abs := filepath.Join(projectRoot, filepath.FromSlash(change.Path))
		switch change.Action {
		case ActionAdd, ActionChange:
			content, ok := byRel[change.Path]
			if !ok {
				return fmt.Errorf("no rendered content for %s", change.Path)
			}
			if err := os.MkdirAll(filepath.Dir(abs), 0755); err != nil {
				return err
			}
			if err := atomicfile.Write(abs, []byte(content), 0644); err != nil {
				return err
			}
		case ActionDelete:
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return err
			}
			_ = os.Remove(filepath.Dir(abs))
		}
	}
	return nil
}

// RenderSelected renders every file for the named agents.
func RenderSelected(projectRoot string, targets []string) ([]File, error) {
	agents := types.DefaultAgents()
	var files []File
	for _, target := range targets {
		agent, ok := agents[target]
		if !ok {
			return nil, fmt.Errorf("unknown agent %q", target)
		}
		rendered, err := Render(agent, projectRoot)
		if err != nil {
			return nil, err
		}
		files = append(files, rendered...)
	}
	return files, nil
}

func isGenerated(content string) bool {
	return strings.Contains(content, "generated: skmgr") || strings.Contains(content, "skmgr:generated")
}

// Restamp recomputes the ownership hash. Tests use it after changing the
// template revision so the file still looks generated rather than edited.
func Restamp(content string) string {
	return stamp(stripHash(content))
}

func stamp(content string) string {
	stripped := stripHash(content)
	sum := sha256.Sum256([]byte(stripped))
	encoded := hex.EncodeToString(sum[:])
	return strings.Replace(stripped, `hash: ""`, "hash: "+encoded, 1)
}

func stripHash(content string) string {
	var lines []string
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "hash:") || strings.HasPrefix(trim, "# hash:") {
			if strings.HasPrefix(trim, "# hash:") {
				lines = append(lines, "# hash: \"\"")
			} else {
				lines = append(lines, strings.Repeat(" ", indentOf(line))+"hash: \"\"")
			}
			continue
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func indentOf(line string) int {
	n := 0
	for _, r := range line {
		if r != ' ' {
			break
		}
		n++
	}
	return n
}

func recordedHash(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		trim = strings.TrimPrefix(trim, "# ")
		if rest, ok := strings.CutPrefix(trim, "hash:"); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`)
		}
	}
	return ""
}

func hashMatches(content string) bool {
	recorded := recordedHash(content)
	if recorded == "" {
		return false
	}
	sum := sha256.Sum256([]byte(stripHash(content)))
	return hex.EncodeToString(sum[:]) == recorded
}

// RevisionOf reads the template revision stored in a generated file.
// The second result is false when the file is not generated.
func RevisionOf(content string) (int, bool) {
	if !isGenerated(content) {
		return 0, false
	}
	for _, line := range strings.Split(content, "\n") {
		trim := strings.TrimSpace(line)
		trim = strings.TrimPrefix(trim, "# ")
		if rest, ok := strings.CutPrefix(trim, "revision:"); ok {
			var n int
			if _, err := fmt.Sscan(strings.TrimSpace(rest), &n); err == nil {
				return n, true
			}
		}
	}
	return 0, true
}

// Edited reports whether a generated file was changed by hand.
func Edited(content string) bool {
	return isGenerated(content) && !hashMatches(content)
}
