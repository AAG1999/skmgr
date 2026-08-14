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

// A frozen install promises the lockfile's exact content. If the source moved
// under a supposedly pinned ref, or the cache was tampered with, saying nothing
// would defeat the whole point of --frozen.
func TestInstallFrozen_FailsWhenContentDoesNotMatchTheLock(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "drifting")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/drifting
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("initial install failed: %v", err)
	}

	// Change the source. A local source has no commit to pin, so the only
	// thing standing between the user and silent drift is the content hash.
	sourceFile := filepath.Join(source, "skills", "drifting", "SKILL.md")
	if err := os.WriteFile(sourceFile, []byte("different content entirely"), 0644); err != nil {
		t.Fatal(err)
	}

	installFrozen = true

	err := installCmd.RunE(installCmd, nil)
	if err == nil {
		t.Fatal("a frozen install should refuse content that does not match the lockfile")
	}
	if !strings.Contains(err.Error(), "drifting") {
		t.Errorf("the error should name the skill, got: %v", err)
	}
	if !strings.Contains(err.Error(), "lockfile") {
		t.Errorf("the error should explain the mismatch, got: %v", err)
	}

	// Drift gets its own exit code so a pipeline can fail a pull request over a
	// changed source without also failing when the network is down.
	if status := exitStatusFor(err); status != ExitDrift {
		t.Errorf("exit status = %d, want %d for drift", status, ExitDrift)
	}

	// Refusing after already overwriting the install would leave the project in
	// exactly the state --frozen was meant to prevent.
	installed, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "drifting", "SKILL.md"))
	if err != nil {
		t.Fatalf("the previous install should still be present: %v", err)
	}
	if strings.Contains(string(installed), "different content entirely") {
		t.Error("the rejected content was installed anyway")
	}
}

// The same drift without --frozen is an ordinary update.
func TestInstall_UnfrozenAcceptsChangedContent(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "drifting")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/drifting
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("initial install failed: %v", err)
	}

	sourceFile := filepath.Join(source, "skills", "drifting", "SKILL.md")
	if err := os.WriteFile(sourceFile, []byte("updated content"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("an unfrozen install should accept the new content: %v", err)
	}

	installed, err := os.ReadFile(filepath.Join(dir, ".agents", "skills", "drifting", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(installed) != "updated content" {
		t.Errorf("installed content = %q, want the update", installed)
	}
}

// A skill added to the manifest but never locked cannot be installed frozen.
func TestInstallFrozen_FailsForASkillMissingFromTheLock(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "first", "second")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/first
`, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("initial install failed: %v", err)
	}

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/first
  - source: file://%s
    path: skills/second
`, source, source))

	installFrozen = true

	err := installCmd.RunE(installCmd, nil)
	if err == nil {
		t.Fatal("a frozen install should refuse a skill that is not in the lockfile")
	}
	if !strings.Contains(err.Error(), "second") {
		t.Errorf("the error should name the unlocked skill, got: %v", err)
	}
}
