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

package workflow

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestRender_AgentPaths(t *testing.T) {
	root := t.TempDir()
	agents := types.DefaultAgents()
	want := map[string][]string{
		"cursor": {
			".cursor/skills/skmgr-find-skill/SKILL.md",
			".cursor/commands/skm-find.md",
		},
		"claude-code": {
			".claude/skills/skmgr-find-skill/SKILL.md",
			".claude/commands/skm/find.md",
		},
		"gemini": {
			".agents/skills/skmgr-find-skill/SKILL.md",
			".gemini/commands/skm/find.toml",
		},
		"copilot": {
			".github/skills/skmgr-find-skill/SKILL.md",
			".github/prompts/skm-find.prompt.md",
		},
	}
	for name, paths := range want {
		files, err := Render(agents[name], root)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, file := range files {
			got[file.Rel] = true
			if !strings.Contains(file.Content, "generated: skmgr") && !strings.Contains(file.Content, "skmgr:generated") {
				t.Errorf("%s %s has no ownership marker", name, file.Rel)
			}
			if _, ok := RevisionOf(file.Content); !ok {
				t.Errorf("%s %s has no revision", name, file.Rel)
			}
		}
		for _, path := range paths {
			if !got[path] {
				t.Errorf("%s missing %s", name, path)
			}
		}
	}
}

func TestPlan_SecondRunIsEmptyAndDryRunWritesNothing(t *testing.T) {
	root := t.TempDir()
	files, err := RenderSelected(root, []string{"cursor"})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := Plan(root, []string{"cursor"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, changes, files); err != nil {
		t.Fatal(err)
	}
	before := snapshot(t, root)
	again, err := Plan(root, []string{"cursor"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("second plan = %#v, want no changes", again)
	}
	if err := Apply(root, again, files); err != nil {
		t.Fatal(err)
	}
	if snapshot(t, root) != before {
		t.Fatal("applying an empty plan changed files")
	}
}

func TestPlan_KeepsEditedFileUnlessForced(t *testing.T) {
	root := t.TempDir()
	files, err := RenderSelected(root, []string{"cursor"})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := Plan(root, []string{"cursor"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, changes, files); err != nil {
		t.Fatal(err)
	}
	skill := filepath.Join(root, ".cursor", "skills", "skmgr-find-skill", "SKILL.md")
	original, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	edited := string(original) + "\nlocal note\n"
	if err := os.WriteFile(skill, []byte(edited), 0644); err != nil {
		t.Fatal(err)
	}
	kept, err := Plan(root, []string{"cursor"}, false)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, change := range kept {
		if strings.HasSuffix(change.Path, "skmgr-find-skill/SKILL.md") {
			found = true
			if change.Action != ActionSkip {
				t.Fatalf("edited file action = %s, want skip", change.Action)
			}
		}
	}
	if !found {
		t.Fatal("edited file was not reported")
	}
	forced, err := Plan(root, []string{"cursor"}, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range forced {
		if strings.HasSuffix(change.Path, "skmgr-find-skill/SKILL.md") && change.Action != ActionChange {
			t.Fatalf("forced action = %s, want change", change.Action)
		}
	}
}

func TestPlan_DropsUnselectedGeneratedFilesOnly(t *testing.T) {
	root := t.TempDir()
	files, err := RenderSelected(root, []string{"cursor", "claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	changes, err := Plan(root, []string{"cursor", "claude-code"}, false)
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, changes, files); err != nil {
		t.Fatal(err)
	}
	edited := filepath.Join(root, ".claude", "skills", "skmgr-update-skill", "SKILL.md")
	body, err := os.ReadFile(edited)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(edited, append(body, []byte("\nkeep me\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	unmarked := filepath.Join(root, ".github", "prompts", "skm-find.prompt.md")
	if err := os.MkdirAll(filepath.Dir(unmarked), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(unmarked, []byte("user prompt\n"), 0644); err != nil {
		t.Fatal(err)
	}
	other := filepath.Join(root, ".github", "PULL_REQUEST_TEMPLATE.md")
	if err := os.WriteFile(other, []byte("unrelated\n"), 0644); err != nil {
		t.Fatal(err)
	}
	claudeOther := filepath.Join(root, ".claude", "settings.json")
	if err := os.WriteFile(claudeOther, []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	next, err := Plan(root, []string{"cursor"}, false)
	if err != nil {
		t.Fatal(err)
	}
	dropped, err := RenderSelected(root, []string{"claude-code"})
	if err != nil {
		t.Fatal(err)
	}
	if err := Apply(root, next, dropped); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, ".claude", "skills", "skmgr-find-skill", "SKILL.md")); !os.IsNotExist(err) {
		t.Fatal("unedited claude skill should be deleted")
	}
	if _, err := os.Stat(edited); err != nil {
		t.Fatal("edited claude skill should stay")
	}
	if _, err := os.Stat(unmarked); err != nil {
		t.Fatal("unmarked copilot prompt should stay")
	}
	if _, err := os.Stat(other); err != nil {
		t.Fatal("unrelated github file should stay")
	}
	if _, err := os.Stat(claudeOther); err != nil {
		t.Fatal("unrelated claude file should stay")
	}
}

func TestTemplates_DoNotAskForAPlanOrArchive(t *testing.T) {
	for name, body := range TemplateSources() {
		for _, line := range strings.Split(strings.ToLower(body), "\n") {
			if strings.Contains(line, "do not") {
				continue
			}
			for _, banned := range []string{"write a plan", "task list", "archive the change", "openspec/changes"} {
				if strings.Contains(line, banned) {
					t.Errorf("%s tells the agent to %q", name, banned)
				}
			}
		}
	}
}

func TestTemplates_CreateInterviewsBeforeWriting(t *testing.T) {
	body := TemplateSources()["create.md"]
	for _, want := range []string{
		"Anthropic's skill creator",
		"Ask in four batches",
		"Which phrases, file types, and contexts should trigger it?",
		"What must it never do?",
		"Wait for confirmation before writing a file.",
		"Do not run skmgr add.",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("create template missing %q", want)
		}
	}
	for _, banned := range []string{"skmgr add ", "skmgr update", "skmgr remove"} {
		if strings.Contains(body, banned) {
			t.Errorf("create template names %q", banned)
		}
	}
	files, err := Render(types.DefaultAgents()["cursor"], t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if !strings.HasSuffix(file.Rel, "skmgr-create-skill/SKILL.md") {
			continue
		}
		if strings.Contains(file.Content, "allowed-tools:") || strings.Contains(file.Content, "Bash(skmgr") {
			t.Fatal("create skill should not be allowed to run skmgr")
		}
	}
}

func TestTemplates_DryRunPrecedesWrite(t *testing.T) {
	for _, name := range []string{"install.md", "update.md", "remove.md"} {
		body := TemplateSources()[name]
		dry := strings.Index(body, "--dry-run")
		if dry < 0 {
			t.Fatalf("%s has no dry run", name)
		}
		rest := body[dry+len("--dry-run"):]
		if !strings.Contains(rest, "skmgr ") {
			t.Fatalf("%s dry run is not followed by the write", name)
		}
		if strings.Contains(rest, "--dry-run") && strings.Index(rest, "--dry-run") < strings.Index(rest, "skmgr ") {
			t.Fatalf("%s shows another dry run before the write", name)
		}
	}
}

func snapshot(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		b.WriteString(rel)
		b.Write(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return b.String()
}
