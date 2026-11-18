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
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/manifest"
)

func decodeError(t *testing.T, err error) (errorDocument, int) {
	t.Helper()
	var buf bytes.Buffer
	status := writeError(&buf, err)
	var doc errorDocument
	if decErr := json.Unmarshal(buf.Bytes(), &doc); decErr != nil {
		t.Fatalf("decoding %s: %v", buf.String(), decErr)
	}
	return doc, status
}

func TestJSON_ErrorCodes(t *testing.T) {
	dir := t.TempDir()
	missing, err := manifest.FindManifestPath(dir)
	if err == nil {
		t.Fatalf("found a manifest at %s", missing)
	}
	doc, status := decodeError(t, err)
	if doc.Error.Code != "no_manifest" || status != ExitError || !strings.Contains(doc.Error.Fix, "skmgr init") {
		t.Fatalf("no_manifest: %+v status %d", doc, status)
	}

	doc, status = decodeError(t, lockfile.ErrNotFound)
	if doc.Error.Code != "no_lockfile" || status != ExitError || !strings.Contains(doc.Error.Fix, "install") {
		t.Fatalf("no_lockfile: %+v status %d", doc, status)
	}

	_, err = manifest.ParseBytes([]byte("skills: ["))
	doc, status = decodeError(t, err)
	if doc.Error.Code != "invalid_manifest" || status != ExitError {
		t.Fatalf("invalid_manifest: %+v status %d (%v)", doc, status, err)
	}

	doc, status = decodeError(t, &engine.HashMismatchError{Name: "demo", Expected: "a", Actual: "b"})
	if doc.Error.Code != "hash_mismatch" || status != ExitDrift {
		t.Fatalf("hash_mismatch: %+v status %d", doc, status)
	}

	doc, status = decodeError(t, &UsageError{Msg: "invalid argument \"nope\" for --targets"})
	if doc.Error.Code != "usage" || status != ExitUsage {
		t.Fatalf("usage: %+v status %d", doc, status)
	}

	var buf bytes.Buffer
	if status := writeError(&buf, &silentError{code: "1 check(s) failed"}); status != ExitDrift || buf.Len() != 0 {
		t.Fatalf("silent error should keep the report and exit 3, got %d %q", status, buf.String())
	}
}

func TestJSON_ChangeDocumentAndDryRun(t *testing.T) {
	dir := newProjectDir(t)
	source := newSkillSource(t, "pdf-tools")
	writeManifest(t, "skmgr.yml", `version: "1"
name: test
targets:
  - cursor
skills: []
`)
	initTargets = []string{"cursor"}
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}

	before := readTree(t, dir)
	jsonOutput = true
	addDryRun = true
	addPath = "skills/pdf-tools"
	var out bytes.Buffer
	addCmd.SetOut(&out)
	installCmd.SetOut(&out)
	removeCmd.SetOut(&out)
	updateCmd.SetOut(&out)
	t.Cleanup(func() {
		addCmd.SetOut(nil)
		installCmd.SetOut(nil)
		removeCmd.SetOut(nil)
		updateCmd.SetOut(nil)
	})
	if err := addCmd.RunE(addCmd, []string{"file://" + source}); err != nil {
		t.Fatal(err)
	}
	var doc changeDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decoding %s: %v", out.String(), err)
	}
	if !doc.DryRun || len(doc.Changes) != 1 || doc.Changes[0].Action != engine.PlanInstall || doc.Changes[0].Name != "pdf-tools" {
		t.Fatalf("plan = %+v", doc)
	}
	if readTree(t, dir) != before {
		t.Fatal("dry run changed the project")
	}

	addDryRun = false
	out.Reset()
	if err := addCmd.RunE(addCmd, []string{"file://" + source}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("decoding %s: %v", out.String(), err)
	}
	if doc.DryRun || len(doc.Changes) != 1 || doc.Changes[0].Action != engine.PlanInstall {
		t.Fatalf("install document = %+v", doc)
	}

	out.Reset()
	installDryRun = false
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Changes == nil || len(doc.Changes) != 0 {
		t.Fatalf("nothing-to-do = %+v", doc)
	}

	installed := filepath.Join(dir, ".agents", "skills", "pdf-tools", "SKILL.md")
	if err := os.WriteFile(installed, []byte("changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err := installCmd.RunE(installCmd, nil); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Changes) != 1 || doc.Changes[0].Action != engine.PlanReinstall {
		t.Fatalf("reinstall = %+v", doc)
	}

	out.Reset()
	if err := removeCmd.RunE(removeCmd, []string{"pdf-tools"}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Changes) != 1 || doc.Changes[0].Action != engine.PlanRemove {
		t.Fatalf("remove = %+v", doc)
	}
}

func TestJSON_InitFileActions(t *testing.T) {
	dir := newProjectDir(t)
	jsonOutput = true
	initDryRun = true
	initTargets = []string{"cursor"}
	var out bytes.Buffer
	initCmd.SetOut(&out)
	t.Cleanup(func() { initCmd.SetOut(nil) })
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	var doc initReport
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%s: %v", out.String(), err)
	}
	if len(doc.Targets) != 1 || doc.Targets[0] != "cursor" {
		t.Fatalf("targets = %v", doc.Targets)
	}
	var added bool
	for _, file := range doc.Files {
		if file.Action == "add" {
			added = true
		}
		if _, err := os.Stat(filepath.Join(dir, filepath.FromSlash(file.Path))); err == nil && file.Action == "add" {
			t.Fatalf("dry run wrote %s", file.Path)
		}
	}
	if !added {
		t.Fatal("expected add actions")
	}
	if _, err := os.Stat(filepath.Join(dir, "skmgr.yml")); !os.IsNotExist(err) {
		t.Fatal("dry run wrote the manifest")
	}
}

func readTree(t *testing.T, root string) string {
	t.Helper()
	var b strings.Builder
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		b.WriteString(rel)
		b.Write(data)
		return nil
	})
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return b.String()
}
