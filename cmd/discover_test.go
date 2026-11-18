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
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/provider"
)

func TestReadDescription_BoundsUntrustedText(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("a", descriptionLimit+50)
	body := "---\ndescription: \"Fills forms\x1b[31m " + long + "\"\n---\n# skill\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	got, truncated := readDescription(dir)
	if !truncated {
		t.Fatal("expected truncation")
	}
	if strings.ContainsAny(got, "\x1b\n\r") {
		t.Fatalf("control characters survived: %q", got)
	}
	if len([]rune(got)) != descriptionLimit {
		t.Fatalf("len = %d", len([]rune(got)))
	}

	plain := t.TempDir()
	if err := os.WriteFile(filepath.Join(plain, "SKILL.md"), []byte("# no frontmatter\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if text, cut := readDescription(plain); text != "" || cut {
		t.Fatalf("missing frontmatter returned %q %v", text, cut)
	}
}

func TestDiscover_ListsSkillsRulesAndRoundTrips(t *testing.T) {
	root := t.TempDir()
	for _, rel := range []string{"skills/pdf-tools", "skills/skill-creator", "rules/review"} {
		if err := os.MkdirAll(filepath.Join(root, rel), 0755); err != nil {
			t.Fatal(err)
		}
	}
	write := func(rel, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, rel), []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("skills/pdf-tools/SKILL.md", "---\ndescription: Fills PDF forms\n---\n# pdf\n")
	write("skills/skill-creator/SKILL.md", "---\ndescription: Creates skills\n---\n# creator\n")
	write("rules/review/AGENTS.md", "# review\n")

	newProjectDir(t)
	jsonOutput = true
	var out bytes.Buffer
	discoverCmd.SetOut(&out)
	t.Cleanup(func() { discoverCmd.SetOut(nil) })

	source := "file://" + root
	if err := discoverCmd.RunE(discoverCmd, []string{source}); err != nil {
		t.Fatal(err)
	}
	var doc discoverDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatalf("%s: %v", out.String(), err)
	}
	if len(doc.Entries) != 3 {
		t.Fatalf("entries = %+v", doc.Entries)
	}
	byPath := map[string]discoverEntry{}
	for _, entry := range doc.Entries {
		byPath[entry.Path] = entry
	}
	if byPath["skills/pdf-tools"].Description != "Fills PDF forms" || byPath["skills/pdf-tools"].Type != "skill" {
		t.Fatalf("pdf = %+v", byPath["skills/pdf-tools"])
	}
	if byPath["rules/review"].Type != "rule" {
		t.Fatalf("rule = %+v", byPath["rules/review"])
	}

	out.Reset()
	discoverPath = "skills/p*"
	if err := discoverCmd.RunE(discoverCmd, []string{source}); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 1 || doc.Entries[0].Name != "pdf-tools" {
		t.Fatalf("glob = %+v", doc.Entries)
	}

	initTargets = []string{"cursor"}
	if err := initCmd.RunE(initCmd, nil); err != nil {
		t.Fatal(err)
	}
	addPath = "skills/pdf-tools"
	addType = "skill"
	jsonOutput = false
	if err := addCmd.RunE(addCmd, []string{source}); err != nil {
		t.Fatal(err)
	}
	m, err := manifest.Parse("skmgr.yml")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Skills) != 1 {
		t.Fatalf("skills = %+v", m.Skills)
	}
	// A derived name is omitted from the manifest. The installed directory is the check.
	if m.Skills[0].Name != "" && m.Skills[0].Name != "pdf-tools" {
		t.Fatalf("name = %q", m.Skills[0].Name)
	}
	if _, err := os.Stat(filepath.Join(".agents", "skills", "pdf-tools")); err != nil {
		t.Fatalf("round trip did not install pdf-tools: %v", err)
	}
}

func TestDiscover_SingleSkillRootAndNothingFound(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "SKILL.md"), []byte("---\ndescription: Solo\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	newProjectDir(t)
	jsonOutput = true
	var out bytes.Buffer
	discoverCmd.SetOut(&out)
	t.Cleanup(func() { discoverCmd.SetOut(nil) })
	if err := discoverCmd.RunE(discoverCmd, []string{"file://" + root}); err != nil {
		t.Fatal(err)
	}
	var doc discoverDocument
	if err := json.Unmarshal(out.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if len(doc.Entries) != 1 || doc.Entries[0].Path != "" {
		t.Fatalf("single = %+v", doc.Entries)
	}

	empty := t.TempDir()
	err := discoverCmd.RunE(discoverCmd, []string{"file://" + empty})
	if err == nil {
		t.Fatal("expected nothing found")
	}
	coded, status := decodeError(t, err)
	if coded.Error.Code != "no_skills_found" || status != ExitError {
		t.Fatalf("code = %+v status %d", coded, status)
	}
}

func TestDiscover_DoesNotTouchTheProject(t *testing.T) {
	project := newProjectDir(t)
	source := newSkillSource(t, "pdf-tools")
	writeManifest(t, "skmgr.yml", "version: \"1\"\nname: test\ntargets: []\nskills: []\n")
	before := readTree(t, project)
	if err := discoverCmd.RunE(discoverCmd, []string{"file://" + source}); err != nil {
		t.Fatal(err)
	}
	if readTree(t, project) != before {
		t.Fatal("discover changed the project")
	}
}

func TestDiscover_ValidatesInput(t *testing.T) {
	newProjectDir(t)
	discoverRef = "--upload-pack=x"
	err := discoverCmd.RunE(discoverCmd, []string{"https://example.test/skills.git"})
	if err == nil || exitStatusFor(err) != ExitUsage {
		t.Fatalf("option-like ref: %v", err)
	}

	discoverRef = ""
	discoverPath = "../../etc"
	err = discoverCmd.RunE(discoverCmd, []string{"file:///tmp"})
	if err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("escaping path: %v", err)
	}

	if err := discoverCmd.Args(discoverCmd, nil); err == nil || exitStatusFor(err) != ExitUsage {
		t.Fatalf("missing source: %v", err)
	}
}

func TestDiscover_OfflineAndVersionRange(t *testing.T) {
	newProjectDir(t)
	t.Setenv("SKMGR_CACHE", t.TempDir())
	jsonOutput = true
	discoverOffline = true
	err := discoverCmd.RunE(discoverCmd, []string{"https://127.0.0.1:1/repo.git"})
	if err == nil {
		t.Fatal("offline uncached source should fail")
	}
	doc, status := decodeError(t, err)
	if doc.Error.Code != "network" || status != ExitError {
		t.Fatalf("offline = %+v status %d (%v)", doc, status, err)
	}

	source := newTaggedSource(t, "versioned", "v1.2.0", "v1.3.1")
	discoverOffline = false
	discoverRef = "^1.2.0"
	var out bytes.Buffer
	discoverCmd.SetOut(&out)
	t.Cleanup(func() { discoverCmd.SetOut(nil) })
	if err := discoverCmd.RunE(discoverCmd, []string{source}); err != nil {
		t.Fatal(err)
	}
	var listed discoverDocument
	if err := json.Unmarshal(out.Bytes(), &listed); err != nil {
		t.Fatal(err)
	}
	if listed.ResolvedRef != "v1.3.1" || listed.CommitSHA == "" {
		t.Fatalf("range = %+v", listed)
	}
	if len(listed.Entries) != 1 || listed.Entries[0].Name != "versioned" {
		t.Fatalf("entries = %+v", listed.Entries)
	}
	_ = provider.FetchOptions{}
}
