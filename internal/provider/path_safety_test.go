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
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestResolveWithinBase_AllowsNestedPaths(t *testing.T) {
	base := t.TempDir()

	got, err := resolveWithinBase(base, "skills/thing")
	if err != nil {
		t.Fatalf("resolveWithinBase() returned error: %v", err)
	}

	want := filepath.Join(base, "skills", "thing")
	if got != want {
		t.Errorf("resolveWithinBase() = %q, want %q", got, want)
	}
}

func TestResolveWithinBase_RejectsEscape(t *testing.T) {
	base := t.TempDir()

	escapes := []string{
		"../outside",
		"../../etc/passwd",
		"skills/../../outside",
	}

	for _, rel := range escapes {
		t.Run(rel, func(t *testing.T) {
			if _, err := resolveWithinBase(base, rel); err == nil {
				t.Fatalf("resolveWithinBase(%q) succeeded, want an escape error", rel)
			}
		})
	}
}

func TestFilterWithinBase_DropsOutsideMatches(t *testing.T) {
	base := t.TempDir()
	outside := t.TempDir()

	inside := filepath.Join(base, "keep")
	matches := []string{inside, filepath.Join(outside, "drop")}

	kept, err := filterWithinBase(base, matches)
	if err != nil {
		t.Fatalf("filterWithinBase() returned error: %v", err)
	}

	if len(kept) != 1 {
		t.Fatalf("filterWithinBase() kept %d entries, want 1: %v", len(kept), kept)
	}
	if kept[0] != inside {
		t.Errorf("kept[0] = %q, want %q", kept[0], inside)
	}
}

// A manifest is user data. A local source pointing at "../.." must not be able
// to pull arbitrary host directories into the install tree.
func TestLocalProvider_RejectsPathEscape(t *testing.T) {
	root := t.TempDir()

	sourceDir := filepath.Join(root, "source")
	secretDir := filepath.Join(root, "secret")
	if err := os.MkdirAll(filepath.Join(sourceDir, "skills", "ok"), 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.MkdirAll(secretDir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(secretDir, "SKILL.md"), []byte("secret"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	p := &LocalProvider{}
	_, err := p.Fetch(types.SkillDependency{
		Name:   "evil",
		Source: sourceDir,
		Path:   "../secret",
	}, t.TempDir(), FetchOptions{})

	if err == nil {
		t.Fatal("Fetch() accepted a path escaping the source root, want error")
	}
	if !strings.Contains(err.Error(), "escapes the source root") {
		t.Errorf("error = %q, want it to mention escaping the source root", err.Error())
	}
}

func TestLocalProvider_AllowsPathInsideSource(t *testing.T) {
	root := t.TempDir()
	skillDir := filepath.Join(root, "skills", "ok")
	if err := os.MkdirAll(skillDir, 0755); err != nil {
		t.Fatalf("setup failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte("ok"), 0644); err != nil {
		t.Fatalf("setup failed: %v", err)
	}

	p := &LocalProvider{}
	res, err := p.Fetch(types.SkillDependency{
		Name:   "ok",
		Source: root,
		Path:   "skills/ok",
	}, t.TempDir(), FetchOptions{})

	if err != nil {
		t.Fatalf("Fetch() returned error for a valid nested path: %v", err)
	}
	if len(res.SourceDirs) != 1 {
		t.Fatalf("SourceDirs length = %d, want 1", len(res.SourceDirs))
	}
}
