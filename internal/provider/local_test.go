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
	"path/filepath"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestLocalProvider_Fetch(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()

	p := &LocalProvider{}

	// Create a dummy skill inside skills/ to satisfy validation
	skillsDir := filepath.Join(dir, "skills", "local-skill")
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillsDir, "SKILL.md"), []byte("# Dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	skill := types.SkillDependency{
		Name:   "local-skill",
		Source: "file://" + dir, // Test file:// stripping
	}

	res, err := p.Fetch(skill, cacheDir, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	// Should return the absolute path to dir, without any copy
	expectedAbs, _ := filepath.Abs(skillsDir)
	if len(res.SourceDirs) != 1 || res.SourceDirs[0] != expectedAbs {
		t.Errorf("SourceDirs = %v, want [%q]", res.SourceDirs, expectedAbs)
	}

	// Local provider does not have a commit SHA
	if res.CommitSHA != "" {
		t.Errorf("CommitSHA = %q, want empty", res.CommitSHA)
	}
}

func TestLocalProvider_Fetch_NotDir(t *testing.T) {
	dir := t.TempDir()
	cacheDir := t.TempDir()

	file := filepath.Join(dir, "SKILL.md")
	if err := os.WriteFile(file, []byte("# Dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	p := &LocalProvider{}
	skill := types.SkillDependency{
		Name:   "local-file",
		Source: file, // Pointing to a file instead of a dir
	}

	_, err := p.Fetch(skill, cacheDir, FetchOptions{})
	if err == nil {
		t.Fatal("Fetch() expected error for non-directory, got nil")
	}
}
