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

func TestGitProvider_Fetch_DefaultBranch(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}

	repoPath, commitSHA := setupLocalGitRepo(t)
	cacheDir := t.TempDir()

	p := &GitProvider{}
	skill := types.SkillDependency{
		Name:   "test-skill",
		Source: repoPath,
	}

	res, err := p.Fetch(skill, cacheDir, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	if len(res.SourceDirs) != 2 {
		t.Fatalf("Fetch() returned %d SourceDirs, want 2 (auto-discovery of both skills)", len(res.SourceDirs))
	}

	if res.CommitSHA != commitSHA {
		t.Errorf("Fetch() returned commit SHA %q, want %q", res.CommitSHA, commitSHA)
	}
}

func TestGitProvider_Fetch_WithSubpath(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}

	repoPath, _ := setupLocalGitRepo(t)
	cacheDir := t.TempDir()

	p := &GitProvider{}
	skill := types.SkillDependency{
		Name:   "subskill",
		Source: repoPath,
		Path:   "skills/subskill",
	}

	res, err := p.Fetch(skill, cacheDir, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	if len(res.SourceDirs) != 1 {
		t.Fatalf("Expected 1 SourceDir, got %d", len(res.SourceDirs))
	}

	// SourceDir should end with 'subskill'
	if filepath.Base(res.SourceDirs[0]) != "subskill" {
		t.Errorf("Expected SourceDir to end with 'subskill', got: %s", res.SourceDirs[0])
	}

	// Verify the file exists in the cache subpath
	if _, err := os.Stat(filepath.Join(res.SourceDirs[0], "SKILL.md")); os.IsNotExist(err) {
		t.Error("SKILL.md not found in fetched source dir subpath")
	}
}

func TestGitProvider_Fetch_InvalidPath(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}

	repoPath, _ := setupLocalGitRepo(t)
	cacheDir := t.TempDir()

	p := &GitProvider{}
	skill := types.SkillDependency{
		Name:   "sub-skill",
		Source: repoPath,
		Path:   "does-not-exist",
	}

	_, err := p.Fetch(skill, cacheDir, FetchOptions{})
	if err == nil {
		t.Fatal("Fetch() expected error for invalid path, got nil")
	}
}

func TestGitProvider_Fetch_WithTag(t *testing.T) {
	if !GitInstalled() {
		t.Skip("git not installed")
	}

	repoPath, _ := setupLocalGitRepo(t)
	cacheDir := t.TempDir()

	p := &GitProvider{}
	skill := types.SkillDependency{
		Name:   "tagged-skill",
		Source: repoPath,
		Ref:    "v1.0.0",
		Path:   "skills/test-skill",
	}

	res, err := p.Fetch(skill, cacheDir, FetchOptions{})
	if err != nil {
		t.Fatalf("Fetch() failed: %v", err)
	}

	if res.CommitSHA == "" {
		t.Error("Fetch() returned empty CommitSHA for tag")
	}
}
