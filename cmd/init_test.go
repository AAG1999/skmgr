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

	"github.com/AAG1999/skmgr/internal/manifest"
)

func readGeneratedManifest(t *testing.T, dir string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, "skmgr.yml"))
	if err != nil {
		t.Fatalf("reading generated manifest: %v", err)
	}
	return string(data)
}

// The README promised auto-detection; init used to emit only commented targets.
func TestInit_DetectsExistingAgentDirectories(t *testing.T) {
	dir := newProjectDir(t)

	for _, d := range []string{".cursor", ".claude"} {
		if err := os.MkdirAll(filepath.Join(dir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	got := readGeneratedManifest(t, dir)

	m, err := manifest.ParseBytes([]byte(got))
	if err != nil {
		t.Fatalf("generated manifest does not parse: %v\n%s", err, got)
	}

	want := []string{"cursor", "claude-code"}
	if len(m.Targets) != len(want) {
		t.Fatalf("Targets = %v, want %v\n%s", m.Targets, want, got)
	}
	for i, target := range want {
		if m.Targets[i] != target {
			t.Errorf("Targets[%d] = %q, want %q", i, m.Targets[i], target)
		}
	}
}

func TestInit_NoAgentsLeavesTargetsCommented(t *testing.T) {
	dir := newProjectDir(t)

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	got := readGeneratedManifest(t, dir)

	m, err := manifest.ParseBytes([]byte(got))
	if err != nil {
		t.Fatalf("generated manifest does not parse: %v\n%s", err, got)
	}
	if len(m.Targets) != 0 {
		t.Errorf("Targets = %v, want none when nothing is detected", m.Targets)
	}
	if !strings.Contains(got, "#   - cursor") {
		t.Errorf("expected commented target hints:\n%s", got)
	}
}

// Discovery prefers skmgr.yml, so writing one beside an existing skmgr.yaml
// would silently shadow it.
func TestInit_RefusesWhenYamlSpellingExists(t *testing.T) {
	dir := newProjectDir(t)

	writeManifest(t, "skmgr.yaml", `version: "1"
name: existing
targets: []
skills: []
`)

	err := initCmd.RunE(initCmd, nil)
	if err == nil {
		t.Fatal("init should refuse to run when skmgr.yaml already exists")
	}
	if !strings.Contains(err.Error(), "skmgr.yaml") {
		t.Errorf("error should name the conflicting file, got: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "skmgr.yml")); !os.IsNotExist(statErr) {
		t.Error("init must not create skmgr.yml when it refuses")
	}
}

func TestInit_CreatesCanonicalDirectories(t *testing.T) {
	dir := newProjectDir(t)

	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	for _, sub := range []string{"skills", "rules"} {
		path := filepath.Join(dir, ".agents", sub)
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("expected %s to exist: %v", path, err)
			continue
		}
		if !info.IsDir() {
			t.Errorf("%s should be a directory", path)
		}
	}
}
