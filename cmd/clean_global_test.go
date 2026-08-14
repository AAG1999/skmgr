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

	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/types"
)

// clean used to ignore --global entirely, leaving no way to clear ~/.agents.
func TestClean_GlobalRemovesHomeTreeAndReleasesLedger(t *testing.T) {
	newProjectDir(t)
	home := os.Getenv("HOME")
	source := newSkillSource(t, "global-skill")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/global-skill
    scope: global
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	canonical := filepath.Join(home, ".agents", "skills", "global-skill")
	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("global skill should be installed first: %v", err)
	}

	globalScope = true
	if err := cleanCmd.RunE(cleanCmd, nil); err != nil {
		t.Fatalf("clean --global failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(home, ".agents")); !os.IsNotExist(err) {
		t.Error("clean --global should have removed ~/.agents")
	}

	ledger, err := lockfile.ReadGlobalLedger(lockfile.GlobalLedgerPath(types.SkmgrHome()))
	if err != nil {
		t.Fatal(err)
	}
	if len(ledger.AllNames()) != 0 {
		t.Errorf("ledger should be empty after a global clean, got %v", ledger.AllNames())
	}
}

// A project-scoped clean must leave the shared global tree alone.
func TestClean_ProjectLeavesGlobalIntact(t *testing.T) {
	newProjectDir(t)
	home := os.Getenv("HOME")

	globalSkill := filepath.Join(home, ".agents", "skills", "keep-me")
	if err := os.MkdirAll(globalSkill, 0755); err != nil {
		t.Fatal(err)
	}

	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills: []
`)

	if err := cleanCmd.RunE(cleanCmd, nil); err != nil {
		t.Fatalf("clean failed: %v", err)
	}

	if _, err := os.Stat(globalSkill); err != nil {
		t.Errorf("a project clean must not touch the global tree: %v", err)
	}
}

// clean never stripped skmgr blocks from merged rule files, leaving sections
// pointing at skills that no longer existed.
func TestClean_StripsManagedRuleBlocks(t *testing.T) {
	dir := newProjectDir(t)

	claudeDir := filepath.Join(dir, ".claude")
	if err := os.MkdirAll(claudeDir, 0755); err != nil {
		t.Fatal(err)
	}
	claudeMD := filepath.Join(claudeDir, "CLAUDE.md")

	contents := strings.Join([]string{
		"# My own instructions",
		"Always run the linter.",
		"",
		"<!-- skmgr:start:team-rule -->",
		"managed rule body",
		"<!-- skmgr:end:team-rule -->",
		"",
		"<!-- skmgr:start:other-rule -->",
		"another managed body",
		"<!-- skmgr:end:other-rule -->",
		"",
		"## Trailing notes",
	}, "\n")

	if err := os.WriteFile(claudeMD, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - claude-code
skills: []
`)

	if err := cleanCmd.RunE(cleanCmd, nil); err != nil {
		t.Fatalf("clean failed: %v", err)
	}

	after, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatal(err)
	}
	got := string(after)

	if strings.Contains(got, "skmgr:start:") {
		t.Errorf("clean left managed blocks behind:\n%s", got)
	}
	if strings.Contains(got, "managed rule body") || strings.Contains(got, "another managed body") {
		t.Errorf("clean left managed content behind:\n%s", got)
	}
	if !strings.Contains(got, "Always run the linter.") {
		t.Errorf("clean removed the user's own content:\n%s", got)
	}
	if !strings.Contains(got, "## Trailing notes") {
		t.Errorf("clean removed content after the managed blocks:\n%s", got)
	}
}
