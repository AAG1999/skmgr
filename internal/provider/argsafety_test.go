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

// The manifest parser refuses these first, but the provider is what actually
// hands the value to git, so it has to refuse them on its own. A caller that
// built a dependency in code rather than parsing a file would otherwise slip
// straight past the only check.
func TestGitFetch_RefusesAnOptionAsTheSource(t *testing.T) {
	t.Setenv("SKMGR_HOME", t.TempDir())

	provider := &GitProvider{}
	_, err := provider.Fetch(
		types.SkillDependency{
			Name:   "hostile",
			Source: "--upload-pack=touch " + filepath.Join(t.TempDir(), "pwned"),
		},
		t.TempDir(),
		FetchOptions{},
	)

	if err == nil {
		t.Fatal("a source that git would read as an option must be refused")
	}
	if !strings.Contains(err.Error(), "option") {
		t.Errorf("the error should explain the refusal, got: %v", err)
	}
}

func TestGitFetch_RefusesAnOptionAsTheRef(t *testing.T) {
	t.Setenv("SKMGR_HOME", t.TempDir())

	source := newTaggedRepo(t, "v1.0.0")

	provider := &GitProvider{}
	_, err := provider.Fetch(
		types.SkillDependency{
			Name:   "hostile",
			Source: source,
			Path:   "skills/versioned",
			Ref:    "--exec=touch " + filepath.Join(t.TempDir(), "pwned"),
		},
		t.TempDir(),
		FetchOptions{},
	)

	if err == nil {
		t.Fatal("a ref that git would read as an option must be refused")
	}
	if !strings.Contains(err.Error(), "option") {
		t.Errorf("the error should explain the refusal, got: %v", err)
	}
}

// The guard must not have made ordinary refs unusable.
func TestGitFetch_StillAcceptsARealRef(t *testing.T) {
	t.Setenv("SKMGR_HOME", t.TempDir())

	source := newTaggedRepo(t, "v1.0.0", "v1.1.0")

	provider := &GitProvider{}
	result, err := provider.Fetch(
		types.SkillDependency{
			Name:   "fine",
			Source: source,
			Path:   "skills/versioned",
			Ref:    "v1.0.0",
		},
		t.TempDir(),
		FetchOptions{},
	)
	if err != nil {
		t.Fatalf("a real tag should still resolve: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(result.SourceDirs[0], "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), "v1.0.0") {
		t.Errorf("checked out the wrong tag:\n%s", content)
	}
}
