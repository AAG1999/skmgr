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
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// commandsWithGlobal are the commands that genuinely act on one scope.
var commandsWithGlobal = []*cobra.Command{addCmd, listCmd, cleanCmd}

// commandsWithoutGlobal take their scope from each manifest entry, so a flag
// would have nothing to act on. Accepting one and ignoring it is worse than
// rejecting it: `skmgr install --global` used to look like it worked.
var commandsWithoutGlobal = []*cobra.Command{installCmd, syncCmd, updateCmd, removeCmd}

func TestGlobalFlag_IsDeclaredWhereItApplies(t *testing.T) {
	for _, cmd := range commandsWithGlobal {
		t.Run(cmd.Name(), func(t *testing.T) {
			flag := cmd.Flags().Lookup("global")
			if flag == nil {
				t.Fatalf("%s should accept --global", cmd.Name())
			}
			if flag.Shorthand != "g" {
				t.Errorf("--global shorthand = %q, want \"g\"", flag.Shorthand)
			}
			if flag.Usage == "" {
				t.Error("--global needs usage text saying what it does here")
			}
		})
	}
}

func TestGlobalFlag_IsAbsentWhereItWouldBeIgnored(t *testing.T) {
	for _, cmd := range commandsWithoutGlobal {
		t.Run(cmd.Name(), func(t *testing.T) {
			if flag := cmd.Flags().Lookup("global"); flag != nil {
				t.Errorf("%s declares --global but takes its scope from the manifest", cmd.Name())
			}
			if flag := cmd.InheritedFlags().Lookup("global"); flag != nil {
				t.Errorf("%s inherits --global, which it silently ignores", cmd.Name())
			}
		})
	}
}

// Rejecting the flag is the point: a user who types it should be told, not
// left with a command that quietly did something else.
func TestGlobalFlag_IsRejectedByInstall(t *testing.T) {
	newProjectDir(t)

	err := installCmd.ParseFlags([]string{"--global"})
	if err == nil {
		t.Fatal("install should reject --global rather than ignore it")
	}
	if !strings.Contains(err.Error(), "unknown flag") {
		t.Errorf("expected an unknown-flag error, got: %v", err)
	}
}

// list --global narrows the table to the skills installed into the home
// directory, which is the only reading of the flag that makes sense there.
func TestListGlobal_ShowsOnlyGlobalSkills(t *testing.T) {
	newProjectDir(t)
	source := newSkillSource(t, "local-one", "shared-one")

	writeManifest(t, "skmgr.yml", fmt.Sprintf(`version: "1"
name: test
targets:
  - cursor
skills:
  - source: file://%s
    path: skills/local-one
  - source: file://%s
    path: skills/shared-one
    scope: global
`, source, source))

	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatalf("install failed: %v", err)
	}

	globalScope = true
	t.Cleanup(func() { globalScope = false })

	output, err := runWithOutput(t, listCmd, nil)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(output, "shared-one") {
		t.Errorf("the global skill should be listed, got:\n%s", output)
	}
	if strings.Contains(output, "local-one") {
		t.Errorf("a project-scoped skill should be filtered out, got:\n%s", output)
	}
}

func TestListGlobal_SaysWhenThereAreNone(t *testing.T) {
	installOneSkill(t, "greeter")

	globalScope = true
	t.Cleanup(func() { globalScope = false })

	buf := captureLog(t)
	if _, err := runWithOutput(t, listCmd, nil); err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if !strings.Contains(buf.String(), "globally scoped") {
		t.Errorf("the message should say the filter found nothing, got: %s", buf.String())
	}
}

// JSON consumers should never have to guard against null.
func TestListJSON_EmptyResultIsAnArray(t *testing.T) {
	installOneSkill(t, "greeter")

	globalScope, listJSON = true, true
	t.Cleanup(func() { globalScope, listJSON = false, false })

	output, err := runWithOutput(t, listCmd, nil)
	if err != nil {
		t.Fatalf("list failed: %v", err)
	}

	if strings.TrimSpace(output) != "[]" {
		t.Errorf("expected an empty array, got: %s", output)
	}
}
