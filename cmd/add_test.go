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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdd_NoManifest(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	cmd := addCmd
	err := cmd.RunE(cmd, []string{"file://dummy"})
	if err == nil {
		t.Fatal("Expected error when no manifest exists")
	}
	if !strings.Contains(err.Error(), "skmgr.yml") {
		t.Error("Expected error to mention skmgr.yml")
	}
}

func TestAdd_AppendsToManifestAndInstalls(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	_ = os.WriteFile("skmgr.yml", []byte(`version: "1"
name: test
targets:
  - cursor
skills: []
`), 0644)

	source := t.TempDir()
	skillsDir := filepath.Join(source, "skills", "my-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("data"), 0644)

	// Reset flags
	addName = "my-skill"
	addPath = ""
	addRef = "main"
	addType = "skill"
	addScope = "project"
	addTargets = nil

	cmd := addCmd
	err := cmd.RunE(cmd, []string{"file://" + source})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	data, _ := os.ReadFile("skmgr.yml")
	if !strings.Contains(string(data), "name: my-skill") {
		t.Error("Manifest was not updated")
	}

	if _, err := os.Stat("skmgr.lock"); os.IsNotExist(err) {
		t.Error("Lockfile was not created")
	}

	if _, err := os.Stat(filepath.Join(".agents", "skills", "my-skill", "SKILL.md")); os.IsNotExist(err) {
		t.Error("Skill was not installed")
	}
}

func TestAdd_DuplicateName(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	_ = os.WriteFile("skmgr.yml", []byte(`version: "1"
name: test
targets: []
skills:
  - name: my-skill
    source: dummy
`), 0644)

	addName = "my-skill"

	cmd := addCmd
	err := cmd.RunE(cmd, []string{"dummy"})
	if err == nil {
		t.Fatal("Expected duplicate name error")
	}
}

func TestAdd_DefaultName(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	_ = os.WriteFile("skmgr.yml", []byte(`version: "1"
name: test
targets: []
skills: []
`), 0644)

	source := t.TempDir()
	skillsDir := filepath.Join(source, "skills", "test-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("data"), 0644)

	addName = ""
	addPath = ""

	cmd := addCmd
	err := cmd.RunE(cmd, []string{"file://" + source})
	if err != nil {
		t.Fatalf("Add failed: %v", err)
	}

	data, _ := os.ReadFile("skmgr.yml")
	base := filepath.Base(source)
	if !strings.Contains(string(data), base) {
		t.Errorf("Expected name to be inferred as %q, got manifest:\n%s", base, string(data))
	}
}
