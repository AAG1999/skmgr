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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// installOneSkill sets up a project with a single installed local skill and
// returns the project directory.
func installOneSkill(t *testing.T, name string) string {
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

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}
	return dir
}

func runVerify(t *testing.T) (string, error) {
	t.Helper()

	var out bytes.Buffer
	verifyCmd.SetOut(&out)
	t.Cleanup(func() { verifyCmd.SetOut(nil) })

	err := verifyCmd.RunE(verifyCmd, nil)
	return out.String(), err
}

func TestVerify_PassesForACleanInstall(t *testing.T) {
	installOneSkill(t, "tidy")

	out, err := runVerify(t)
	if err != nil {
		t.Fatalf("verify should pass on a clean install: %v\n%s", err, out)
	}
	if !strings.Contains(out, "ok") {
		t.Errorf("expected an ok status:\n%s", out)
	}
}

// The point of the content hash: local edits to an installed skill mean you are
// no longer running what the lockfile pins.
func TestVerify_DetectsModifiedContent(t *testing.T) {
	dir := installOneSkill(t, "tampered")

	skillFile := filepath.Join(dir, ".agents", "skills", "tampered", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("locally edited"), 0644); err != nil {
		t.Fatal(err)
	}

	out, err := runVerify(t)
	if err == nil {
		t.Fatalf("verify should fail when content was modified:\n%s", out)
	}
	if !strings.Contains(out, "modified") {
		t.Errorf("expected a modified status:\n%s", out)
	}
	if !strings.Contains(err.Error(), "1 modified") {
		t.Errorf("error should count the modifications, got: %v", err)
	}
}

// A pipeline branches on the exit code, so drift has to be distinguishable
// from skmgr failing to run at all.
func TestVerify_DriftExitsWithItsOwnStatus(t *testing.T) {
	dir := installOneSkill(t, "drifted")

	skillFile := filepath.Join(dir, ".agents", "skills", "drifted", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("locally edited"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := runVerify(t)
	if err == nil {
		t.Fatal("verify should fail when content was modified")
	}

	var silent *silentError
	if !errors.As(err, &silent) {
		t.Fatalf("verify should exit quietly after printing its table, got %T", err)
	}
	if silent.ExitStatus() != ExitDrift {
		t.Errorf("exit status = %d, want %d for drift", silent.ExitStatus(), ExitDrift)
	}
}

func TestVerify_DetectsMissingInstall(t *testing.T) {
	dir := installOneSkill(t, "vanished")

	if err := os.RemoveAll(filepath.Join(dir, ".agents", "skills", "vanished")); err != nil {
		t.Fatal(err)
	}

	out, err := runVerify(t)
	if err == nil {
		t.Fatalf("verify should fail when a locked skill is not installed:\n%s", out)
	}
	if !strings.Contains(out, "missing") {
		t.Errorf("expected a missing status:\n%s", out)
	}
	if !strings.Contains(err.Error(), "1 missing") {
		t.Errorf("error should count what is missing, got: %v", err)
	}
}

func TestVerify_RequiresALockfile(t *testing.T) {
	newProjectDir(t)

	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills: []
`)

	_, err := runVerify(t)
	if err == nil {
		t.Fatal("verify should refuse to run without a lockfile")
	}
	if !strings.Contains(err.Error(), LockfileName) {
		t.Errorf("error should name the lockfile, got: %v", err)
	}
}

func TestVerify_JSONOutput(t *testing.T) {
	dir := installOneSkill(t, "json-check")

	skillFile := filepath.Join(dir, ".agents", "skills", "json-check", "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("changed"), 0644); err != nil {
		t.Fatal(err)
	}

	verifyJSON = true

	out, err := runVerify(t)
	if err == nil {
		t.Fatal("expected a failure for modified content")
	}

	var rows []verifyRow
	if decodeErr := json.Unmarshal([]byte(out), &rows); decodeErr != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", decodeErr, out)
	}
	if len(rows) != 1 {
		t.Fatalf("expected one row, got %d", len(rows))
	}
	if rows[0].Status != "modified" {
		t.Errorf("Status = %q, want modified", rows[0].Status)
	}
	if rows[0].Expected == "" || rows[0].Actual == "" {
		t.Error("both hashes should be reported so the user can see the difference")
	}
	if rows[0].Expected == rows[0].Actual {
		t.Error("expected and actual hashes should differ for modified content")
	}
}

// verify must not reach the network, so it has to work with the cache gone.
func TestVerify_WorksWithoutTheCache(t *testing.T) {
	installOneSkill(t, "cacheless")

	home := os.Getenv("HOME")
	if err := os.RemoveAll(filepath.Join(home, ".skmgr", "cache")); err != nil {
		t.Fatal(err)
	}

	if _, err := runVerify(t); err != nil {
		t.Errorf("verify should not need the cache: %v", err)
	}
}
