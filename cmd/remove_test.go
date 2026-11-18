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
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemove_NonExistent(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	_ = os.WriteFile("skmgr.yml", []byte(`version: "1"
name: test
targets: []
skills: []
`), 0644)

	cmd := removeCmd
	err := cmd.RunE(cmd, []string{"my-skill"})
	if err == nil {
		t.Fatal("Expected error for non-existent skill")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected not found error, got: %v", err)
	}
}

func TestRemove_Success(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "my-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/my-skill
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	canonical := filepath.Join(dir, ".agents", "skills", "my-skill")
	linkPath := filepath.Join(dir, ".cursor", "skills", "my-skill")
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("install did not create the skill directory: %v", err)
	}
	if _, err := os.Lstat(linkPath); err != nil {
		t.Fatalf("install did not create the agent link: %v", err)
	}

	if err := removeCmd.RunE(removeCmd, []string{"my-skill"}); err != nil {
		t.Fatalf("Remove failed: %v", err)
	}

	data, err := os.ReadFile("skmgr.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "my-skill") {
		t.Error("Skill was not removed from manifest")
	}

	if _, err := os.Stat(canonical); !os.IsNotExist(err) {
		t.Error("Skill directory was not cleaned up")
	}

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("Skill symlink was not cleaned up")
	}
}
