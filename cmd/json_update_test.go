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
	"bytes"
	"encoding/json"
	"testing"

	"github.com/AAG1999/skmgr/internal/engine"
)

func TestJSON_UpdateReportsTheMove(t *testing.T) {
	newProjectDir(t)
	root, source := taggedSourceRoot(t)
	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills:
  - source: `+source+`
    path: skills/tool
    ref: ^1.0.0
`)
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatal(err)
	}
	tagSourceVersion(t, root, "tool", "v1.2.0")

	jsonOutput = true
	var out bytes.Buffer
	updateCmd.SetOut(&out)
	t.Cleanup(func() { updateCmd.SetOut(nil) })
	if err := updateCmd.RunE(updateCmd, nil); err != nil {
		t.Fatal(err)
	}
	var doc changeDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%s: %v", out.String(), err)
	}
	if doc.DryRun || len(doc.Changes) != 1 || doc.Changes[0].Action != engine.PlanUpdate {
		t.Fatalf("update = %+v", doc)
	}
	if doc.Changes[0].FromRef == "" || doc.Changes[0].ToRef == "" {
		t.Fatalf("refs = %+v", doc.Changes[0])
	}
}

// taggedSourceRoot is newTaggedSource plus the directory later releases are committed to.
func taggedSourceRoot(t *testing.T) (string, string) {
	t.Helper()
	source := newTaggedSource(t, "tool", "v1.0.0", "v1.1.0")
	return source[len("git::"):], source
}
