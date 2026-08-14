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

package provider

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// setupLocalGitRepo creates a dummy local git repository for testing.
// It returns the path to the repo and the commit SHA of the initial commit.
func setupLocalGitRepo(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()

	// Initialize git repo
	cmd := exec.Command("git", "init", "--initial-branch=main")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git init failed: %v", err)
	}

	// Configure git for the dummy repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config email failed: %v", err)
	}
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git config name failed: %v", err)
	}

	// Create a dummy file
	skillsDir := filepath.Join(dir, "skills", "test-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	skillFile := filepath.Join(skillsDir, "SKILL.md")
	if err := os.WriteFile(skillFile, []byte("# Dummy Skill\n"), 0644); err != nil {
		t.Fatalf("writing dummy file failed: %v", err)
	}

	// Commit the file
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "Initial commit")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Create a subdirectory for path testing
	subDir := filepath.Join(dir, "skills", "subskill")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(subDir, "SKILL.md"), []byte("# Sub Skill\n"), 0644); err != nil {
		t.Fatalf("writing subskill file failed: %v", err)
	}

	// Commit subskill
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git add failed: %v", err)
	}
	cmd = exec.Command("git", "commit", "-m", "Add subskill")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git commit failed: %v", err)
	}

	// Get the latest commit SHA
	cmd = exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse failed: %v", err)
	}

	// Create a tag
	cmd = exec.Command("git", "tag", "v1.0.0")
	cmd.Dir = dir
	if err := cmd.Run(); err != nil {
		t.Fatalf("git tag failed: %v", err)
	}

	return dir, string(out[:len(out)-1]) // remove trailing newline
}
