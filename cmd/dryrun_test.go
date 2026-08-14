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

	"github.com/spf13/cobra"
)

// runWithOutput executes a command's RunE with its output captured.
func runWithOutput(t *testing.T, cmd *cobra.Command, args []string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	cmd.SetOut(&out)
	t.Cleanup(func() { cmd.SetOut(nil) })

	err := cmd.RunE(cmd, args)
	return out.String(), err
}

// projectWithManifest sets up a project declaring one local skill, without
// installing it.
func projectWithManifest(t *testing.T, name string) string {
	t.Helper()

	dir := newProjectDir(t)
	source := newSkillSource(t, name)

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/%s
`, source, name))

	return dir
}

// The contract of a dry run is that nothing on disk changes.
func TestInstallDryRun_ChangesNothing(t *testing.T) {
	dir := projectWithManifest(t, "greeter")

	installDryRun = true
	t.Cleanup(func() { installDryRun = false })

	output, err := runWithOutput(t, installCmd, nil)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !strings.Contains(output, "install") || !strings.Contains(output, "greeter") {
		t.Errorf("the plan should say greeter would be installed, got:\n%s", output)
	}

	if _, err := os.Stat(filepath.Join(dir, LockfileName)); !os.IsNotExist(err) {
		t.Error("a dry run must not write a lockfile")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "greeter")); !os.IsNotExist(err) {
		t.Error("a dry run must not install anything")
	}
}

func TestInstallDryRun_ReportsNothingToDo(t *testing.T) {
	installOneSkill(t, "greeter")

	installDryRun = true
	t.Cleanup(func() { installDryRun = false })

	output, err := runWithOutput(t, installCmd, nil)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !strings.Contains(output, "Nothing to do") {
		t.Errorf("an up-to-date project should report no work, got:\n%s", output)
	}
}

// An install that has drifted from the lockfile would be rewritten, and the
// plan has to say so rather than claiming there is nothing to do.
func TestInstallDryRun_NoticesAModifiedInstall(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	installed := filepath.Join(dir, ".agents", "skills", "greeter", "SKILL.md")
	if err := os.WriteFile(installed, []byte("# tampered\n"), 0644); err != nil {
		t.Fatal(err)
	}

	installDryRun = true
	t.Cleanup(func() { installDryRun = false })

	output, err := runWithOutput(t, installCmd, nil)
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !strings.Contains(output, "reinstall") {
		t.Errorf("a modified install should be reported as a reinstall, got:\n%s", output)
	}
}

func TestRemoveDryRun_LeavesTheManifestAlone(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	removeDryRun = true
	t.Cleanup(func() { removeDryRun = false })

	output, err := runWithOutput(t, removeCmd, []string{"greeter"})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !strings.Contains(output, "remove") {
		t.Errorf("the plan should say greeter would be removed, got:\n%s", output)
	}

	manifest, err := os.ReadFile(filepath.Join(dir, "skmgr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(manifest), "greeter") {
		t.Error("a dry run must not rewrite the manifest")
	}
	if _, err := os.Stat(filepath.Join(dir, ".agents", "skills", "greeter")); err != nil {
		t.Error("a dry run must not delete the installed skill")
	}
}

func TestAddDryRun_DoesNotTouchTheManifest(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "greeter")

	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills: []
`)

	addDryRun = true
	addPath = "skills/greeter"
	t.Cleanup(func() { addDryRun, addPath = false, "" })

	output, err := runWithOutput(t, addCmd, []string{"file://" + source})
	if err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if !strings.Contains(output, "greeter") {
		t.Errorf("the plan should mention the skill being added, got:\n%s", output)
	}

	manifest, err := os.ReadFile(filepath.Join(dir, "skmgr.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(manifest), "greeter") {
		t.Error("a dry run must not add the skill to the manifest")
	}
}

func TestSyncDryRun_ChangesNothing(t *testing.T) {
	dir := projectWithManifest(t, "greeter")

	syncDryRun = true
	t.Cleanup(func() { syncDryRun = false })

	if _, err := runWithOutput(t, syncCmd, nil); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, LockfileName)); !os.IsNotExist(err) {
		t.Error("a dry run must not write a lockfile")
	}
}

func TestUpdateDryRun_ChangesNothing(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	before, err := os.ReadFile(filepath.Join(dir, LockfileName))
	if err != nil {
		t.Fatal(err)
	}

	updateDryRun = true
	t.Cleanup(func() { updateDryRun = false })

	if _, err := runWithOutput(t, updateCmd, nil); err != nil {
		t.Fatalf("dry run failed: %v", err)
	}

	after, err := os.ReadFile(filepath.Join(dir, LockfileName))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("a dry run must not rewrite the lockfile")
	}
}

func TestShortSHA(t *testing.T) {
	cases := map[string]string{
		"":    "",
		"abc": "abc",
		"0123456789abcdef0123456789abcdef01234567": "0123456",
	}

	for input, want := range cases {
		if got := shortSHA(input); got != want {
			t.Errorf("shortSHA(%q) = %q, want %q", input, got, want)
		}
	}
}
