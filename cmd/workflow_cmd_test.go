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

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/AAG1999/skmgr/internal/workflow"
)

func TestInit_NoTTYWithoutTargetsExitsUsage(t *testing.T) {
	dir := newProjectDir(t)
	initTerminal = func() bool { return false }
	err := initCmd.RunE(initCmd, nil)
	if exitStatusFor(err) != ExitUsage || !strings.Contains(err.Error(), "--targets") {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "skmgr.yml")); !os.IsNotExist(statErr) {
		t.Fatal("failed init wrote a manifest")
	}
}

func TestInit_UnknownTargetWritesNothing(t *testing.T) {
	dir := newProjectDir(t)
	initTargets = []string{"not-an-agent"}
	err := initCmd.RunE(initCmd, nil)
	if exitStatusFor(err) != ExitUsage {
		t.Fatalf("got %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "skmgr.yml")); !os.IsNotExist(statErr) {
		t.Fatal("unknown target wrote a manifest")
	}
}

func TestInit_SecondRunKeepsSkillsAndLock(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "pdf-tools")
	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills:
  - name: pdf-tools
    source: file://`+source+`
    path: skills/pdf-tools
`)
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatal(err)
	}
	lockBefore, err := os.ReadFile(filepath.Join(dir, "skmgr.lock"))
	if err != nil {
		t.Fatal(err)
	}
	initTargets = []string{"cursor", "gemini"}
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Parse(filepath.Join(dir, "skmgr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Skills) != 1 || m.Skills[0].Name != "pdf-tools" {
		t.Fatalf("skills = %+v", m.Skills)
	}
	if len(m.Targets) != 2 {
		t.Fatalf("targets = %v", m.Targets)
	}
	lockAfter, err := os.ReadFile(filepath.Join(dir, "skmgr.lock"))
	if err != nil {
		t.Fatal(err)
	}
	if string(lockBefore) != string(lockAfter) {
		t.Fatal("init changed skmgr.lock")
	}
	_ = lockfile.ErrNotFound
}

func TestInit_PrintsGettingStarted(t *testing.T) {
	newProjectDir(t)
	initTargets = []string{"cursor"}
	var out bytes.Buffer
	initCmd.SetOut(&out)
	t.Cleanup(func() { initCmd.SetOut(nil) })
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, name := range []string{"/skmgr-find-skill", "/skmgr-install-skill", "/skmgr-update-skill", "/skmgr-remove-skill", "/skmgr-create-skill"} {
		if !strings.Contains(text, name) {
			t.Errorf("missing %s in %s", name, text)
		}
	}
	if !strings.Contains(text, "Restart your IDE to refresh skills.") {
		t.Fatalf("missing restart line:\n%s", text)
	}
}

func TestWorkflow_EndToEndLeavesFilesAndSkills(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "pdf-tools")
	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets: []
skills:
  - name: pdf-tools
    source: file://`+source+`
    path: skills/pdf-tools
`)
	initTargets = []string{"cursor", "gemini", "claude-code", "copilot"}
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	initCmd.SetOut(&out)
	t.Cleanup(func() { initCmd.SetOut(nil) })
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "add ") || strings.Contains(out.String(), "change ") {
		t.Fatalf("second init was not idempotent:\n%s", out.String())
	}
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(dir, ".agents", "skills", "skmgr-find-skill", "SKILL.md")
	cursorSkill := filepath.Join(dir, ".cursor", "skills", "skmgr-find-skill", "SKILL.md")
	if err := cleanCmd.RunE(cleanCmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatal("clean removed the gemini workflow skill")
	}
	if _, err := os.Stat(cursorSkill); err != nil {
		t.Fatal("clean removed the cursor workflow skill")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "pdf-tools")); !os.IsNotExist(err) {
		t.Fatal("clean left the installed skill")
	}
	m, err := manifest.Parse(filepath.Join(dir, "skmgr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Skills) != 1 || m.Skills[0].Name != "pdf-tools" {
		t.Fatalf("skills after init and clean = %+v", m.Skills)
	}
}

func TestClean_KeepsReservedAndDropsInstalls(t *testing.T) {
	dir := installOneSkill(t, "greeter")
	initTargets = []string{"cursor", "gemini"}
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	rules := filepath.Join(dir, ".agents", "rules", "note")
	if err := os.MkdirAll(rules, 0755); err != nil {
		t.Fatal(err)
	}
	if err := cleanCmd.RunE(cleanCmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "greeter")); !os.IsNotExist(err) {
		t.Fatal("install survived clean")
	}
	if _, err := os.Stat(rules); !os.IsNotExist(err) {
		t.Fatal("rule survived clean")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "skmgr-find-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".cursor", "skills", "skmgr-find-skill", "SKILL.md")); err != nil {
		t.Fatal(err)
	}
}

func TestDoctor_WorkflowStates(t *testing.T) {
	dir := newProjectDir(t)
	writeManifest(t, "skmgr.yml", "version: \"1\"\nname: test\ntargets:\n  - cursor\nskills: []\n")
	results, err := runDoctorJSON(t)
	if err == nil {
		t.Fatal("missing workflow files should fail")
	}
	if checkByName(t, results, "workflow").Remedy != "run skmgr init" {
		t.Fatalf("remedy = %q", checkByName(t, results, "workflow").Remedy)
	}
	var silent *silentError
	if !errorsAs(err, &silent) || silent.ExitStatus() != ExitDrift {
		t.Fatalf("exit = %v", err)
	}

	initTargets = []string{"cursor"}
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := runDoctorJSON(t); err != nil {
		t.Fatalf("current files should pass: %v", err)
	}

	skill := filepath.Join(dir, ".cursor", "skills", "skmgr-find-skill", "SKILL.md")
	body, err := os.ReadFile(skill)
	if err != nil {
		t.Fatal(err)
	}
	current := fmt.Sprintf("revision: %d", workflow.TemplateRevision)
	stale := strings.Replace(string(body), current, "revision: 0", 1)
	if err := os.WriteFile(skill, []byte(workflow.Restamp(stale)), 0644); err != nil {
		t.Fatal(err)
	}
	results, err = runDoctorJSON(t)
	if err == nil || checkByName(t, results, "workflow").State != checkFail {
		t.Fatalf("stale = %v %v", err, results)
	}

	if err := os.WriteFile(skill, append(body, []byte("\nhand edit\n")...), 0644); err != nil {
		t.Fatal(err)
	}
	results, err = runDoctorJSON(t)
	if err != nil {
		t.Fatalf("an edited file is not a doctor failure: %v", err)
	}
	if checkByName(t, results, "workflow").State != checkPass {
		t.Fatalf("edited = %+v", checkByName(t, results, "workflow"))
	}
}

func errorsAs(err error, target **silentError) bool {
	if err == nil {
		return false
	}
	s, ok := err.(*silentError)
	if ok {
		*target = s
	}
	return ok
}

func TestWorkflow_FindPermissionsAndSharedBody(t *testing.T) {
	root := t.TempDir()
	files, err := workflow.Render(types.DefaultAgents()["cursor"], root)
	if err != nil {
		t.Fatal(err)
	}
	var skill, command string
	for _, file := range files {
		if strings.HasSuffix(file.Rel, "skmgr-find-skill/SKILL.md") {
			skill = file.Content
		}
		if file.Rel == ".cursor/commands/skm-find.md" {
			command = file.Content
		}
	}
	if !strings.Contains(skill, "Bash(skmgr discover:*)") {
		t.Fatal("find skill cannot discover")
	}
	for _, banned := range []string{"Bash(skmgr add:*)", "Bash(skmgr remove:*)", "Bash(skmgr install:*)", "Bash(skmgr update:*)", "Bash(skmgr init:*)"} {
		if strings.Contains(skill, banned) {
			t.Fatalf("find skill allows %s", banned)
		}
	}
	if bodyAfter(skill) != bodyAfter(command) {
		t.Fatal("skill and command bodies differ")
	}
}

func bodyAfter(content string) string {
	if i := strings.Index(content, "\n---\n"); i >= 0 {
		return content[i+len("\n---\n"):]
	}
	return content
}

func TestWorkflow_CLIDrift(t *testing.T) {
	for name, body := range workflow.TemplateSources() {
		if err := checkSkillCommands(body); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
	if err := checkSkillCommands("skmgr add --not-a-real-flag"); err == nil {
		t.Fatal("an unknown flag should fail the drift check")
	}
}

func checkSkillCommands(body string) error {
	for _, line := range strings.Split(body, "\n") {
		trim := strings.TrimSpace(line)
		if !strings.HasPrefix(trim, "skmgr ") {
			continue
		}
		fields := strings.Fields(trim)
		if len(fields) < 2 {
			continue
		}
		cmd, _, err := rootCmd.Find([]string{fields[1]})
		if err != nil {
			return err
		}
		for _, field := range fields[2:] {
			if !strings.HasPrefix(field, "--") {
				continue
			}
			name := strings.TrimPrefix(strings.Split(field, "=")[0], "--")
			if cmd.Flags().Lookup(name) == nil && rootCmd.PersistentFlags().Lookup(name) == nil {
				return unknownFlagError{command: fields[1], flag: name}
			}
		}
	}
	return nil
}

type unknownFlagError struct {
	command string
	flag    string
}

func (e unknownFlagError) Error() string {
	return "unknown flag --" + e.flag + " on " + e.command
}
