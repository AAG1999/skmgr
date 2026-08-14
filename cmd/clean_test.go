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
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestCleanCmd_Project(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	// Create fake .agents dir
	agentsDir := filepath.Join(dir, ".agents", "skills", "dummy")
	_ = os.MkdirAll(agentsDir, 0755)

	// Create fake broken link in .cursor/skills
	cursorDir := filepath.Join(dir, ".cursor", "skills")
	_ = os.MkdirAll(cursorDir, 0755)
	linkPath := filepath.Join(cursorDir, "dummy")
	_ = os.Symlink(filepath.Join("..", "..", ".agents", "skills", "dummy"), linkPath)

	// Ensure flags are reset
	cleanCache = false
	cleanAll = false

	cmd := cleanCmd
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents")); !os.IsNotExist(err) {
		t.Error(".agents directory was not removed")
	}

	if _, err := os.Lstat(linkPath); !os.IsNotExist(err) {
		t.Error("Broken symlink was not cleaned up")
	}
}

func TestCleanCmd_Cache(t *testing.T) {
	dir := t.TempDir()

	// Override cache dir for testing
	t.Setenv("SKMGR_CACHE", filepath.Join(dir, "cache"))
	cacheDir := types.CacheDir()

	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "test.txt"), []byte("data"), 0644)

	// Ensure flags are reset
	cleanCache = true
	cleanAll = false

	cmd := cleanCmd
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("Cache directory was not removed")
	}
}

func TestCleanCmd_All(t *testing.T) {
	dir := t.TempDir()
	originalWD, _ := os.Getwd()
	_ = os.Chdir(dir)
	defer func() { _ = os.Chdir(originalWD) }()

	// Override cache dir for testing
	t.Setenv("SKMGR_CACHE", filepath.Join(dir, "cache"))
	cacheDir := types.CacheDir()

	// Create fake cache
	_ = os.MkdirAll(cacheDir, 0755)
	_ = os.WriteFile(filepath.Join(cacheDir, "test.txt"), []byte("data"), 0644)

	// Create fake .agents dir
	agentsDir := filepath.Join(dir, ".agents", "skills", "dummy")
	_ = os.MkdirAll(agentsDir, 0755)

	// Ensure flags are set for all
	cleanCache = false
	cleanAll = true

	cmd := cleanCmd
	err := cmd.RunE(cmd, []string{})
	if err != nil {
		t.Fatalf("Clean failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".agents")); !os.IsNotExist(err) {
		t.Error(".agents directory was not removed")
	}

	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Error("Cache directory was not removed")
	}
}
