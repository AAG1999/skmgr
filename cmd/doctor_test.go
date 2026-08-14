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
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// runDoctor executes the command and captures its output.
func runDoctor(t *testing.T) (string, error) {
	t.Helper()

	var out bytes.Buffer
	doctorCmd.SetOut(&out)
	t.Cleanup(func() { doctorCmd.SetOut(nil) })

	err := doctorCmd.RunE(doctorCmd, nil)
	return out.String(), err
}

// checkByName finds one result in the JSON report.
func checkByName(t *testing.T, results []checkResult, name string) checkResult {
	t.Helper()

	for _, result := range results {
		if result.Name == name {
			return result
		}
	}

	t.Fatalf("no %q check in the report", name)
	return checkResult{}
}

// runDoctorJSON executes the command in JSON mode and decodes the report.
func runDoctorJSON(t *testing.T) ([]checkResult, error) {
	t.Helper()

	doctorJSON = true
	t.Cleanup(func() { doctorJSON = false })

	output, err := runDoctor(t)

	var results []checkResult
	if decodeErr := json.Unmarshal([]byte(output), &results); decodeErr != nil {
		t.Fatalf("decoding the report: %v\n%s", decodeErr, output)
	}

	return results, err
}

func TestDoctor_HealthyProject(t *testing.T) {
	installOneSkill(t, "greeter")

	output, err := runDoctor(t)
	if err != nil {
		t.Fatalf("a healthy project should pass: %v\n%s", err, output)
	}

	if !strings.Contains(output, "Everything looks healthy") {
		t.Errorf("expected a clean bill of health, got:\n%s", output)
	}
	for _, check := range []string{"git", "manifest", "store", "symlinks", "lockfile", "installed"} {
		if !strings.Contains(output, check) {
			t.Errorf("the report should cover %q, got:\n%s", check, output)
		}
	}
}

// A skill edited in place no longer matches the lockfile, which is the whole
// reason doctor exists: the agent is reading something the team did not pin.
func TestDoctor_ReportsModifiedSkills(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	installed := filepath.Join(dir, ".agents", "skills", "greeter", "SKILL.md")
	if err := os.WriteFile(installed, []byte("# tampered\n"), 0644); err != nil {
		t.Fatal(err)
	}

	results, err := runDoctorJSON(t)
	if err == nil {
		t.Fatal("doctor should fail when an install no longer matches the lockfile")
	}

	installedCheck := checkByName(t, results, "installed")
	if installedCheck.State != checkFail {
		t.Errorf("installed check = %q, want %q", installedCheck.State, checkFail)
	}
	if !strings.Contains(installedCheck.Detail, "greeter") {
		t.Errorf("the detail should name the skill, got: %s", installedCheck.Detail)
	}
	if installedCheck.Remedy == "" {
		t.Error("a failing check should say what to do about it")
	}
}

func TestDoctor_ReportsMissingSkills(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	if err := os.RemoveAll(filepath.Join(dir, ".agents", "skills", "greeter")); err != nil {
		t.Fatal(err)
	}

	results, err := runDoctorJSON(t)
	if err == nil {
		t.Fatal("doctor should fail when a locked skill is not installed")
	}

	installed := checkByName(t, results, "installed")
	if installed.State != checkFail {
		t.Errorf("installed check = %q, want %q", installed.State, checkFail)
	}
	if !strings.Contains(installed.Remedy, "install") {
		t.Errorf("the remedy should point at install, got: %s", installed.Remedy)
	}
}

// Running outside a project is a common mistake, and should be explained
// rather than crashing or reporting a healthy machine.
func TestDoctor_WarnsWithoutAManifest(t *testing.T) {
	newProjectDir(t)

	results, err := runDoctorJSON(t)
	if err != nil {
		t.Fatalf("a missing manifest is a warning, not a failure: %v", err)
	}

	manifest := checkByName(t, results, "manifest")
	if manifest.State != checkWarn {
		t.Errorf("manifest check = %q, want %q", manifest.State, checkWarn)
	}
	if !strings.Contains(manifest.Remedy, "init") {
		t.Errorf("the remedy should point at init, got: %s", manifest.Remedy)
	}
}

// A project with a manifest but no install yet is incomplete, not broken.
func TestDoctor_WarnsWithoutALockfile(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "greeter")
	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://`+source+`
    path: skills/greeter
`)

	results, err := runDoctorJSON(t)
	if err != nil {
		t.Fatalf("a missing lockfile is a warning, not a failure: %v", err)
	}

	lock := checkByName(t, results, "lockfile")
	if lock.State != checkWarn {
		t.Errorf("lockfile check = %q, want %q", lock.State, checkWarn)
	}
}

// A config file that cannot be parsed has to be reported, since every other
// setting the user thinks is in effect is silently absent.
func TestDoctor_ReportsABrokenConfigFile(t *testing.T) {
	newProjectDir(t)

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte("cache_dir: [unterminated\n"), 0644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("SKMGR_CONFIG", path)
	resetConfigCache(t)

	results, _ := runDoctorJSON(t)

	cfg := checkByName(t, results, "config")
	if cfg.State != checkFail {
		t.Errorf("config check = %q, want %q", cfg.State, checkFail)
	}
}

// The report is the explanation, so Execute must not print a second error
// line underneath it.
func TestDoctor_FailureIsSilent(t *testing.T) {
	dir := installOneSkill(t, "greeter")

	if err := os.RemoveAll(filepath.Join(dir, ".agents", "skills", "greeter")); err != nil {
		t.Fatal(err)
	}

	_, err := runDoctor(t)
	if err == nil {
		t.Fatal("expected a failure")
	}

	var silent *silentError
	if !errors.As(err, &silent) {
		t.Fatalf("expected a silentError so the report is not followed by a redundant message, got %T", err)
	}
	if silent.ExitStatus() != ExitDrift {
		t.Errorf("exit status = %d, want %d for a project that is out of line", silent.ExitStatus(), ExitDrift)
	}
}
