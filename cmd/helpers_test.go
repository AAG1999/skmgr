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
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/config"
	"github.com/AAG1999/skmgr/internal/log"
)

// newProjectDir creates an empty project directory, makes it the working
// directory, and points HOME at a temp dir so global-scope writes and the
// skmgr state directory never touch the developer's real home.
func newProjectDir(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// Point the config file somewhere that does not exist, so a real one on the
	// developer's machine cannot change what these tests observe. The parsed
	// file is cached, so the cache has to be dropped after moving it.
	t.Setenv(config.EnvConfig, filepath.Join(home, "absent-config.yml"))
	config.Reset()
	t.Cleanup(config.Reset)

	dir := t.TempDir()

	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("getting working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("changing to project directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(original) })

	resetCommandFlags(t)

	return dir
}

// resetCommandFlags restores the package-level flag variables between tests,
// since cobra binds them globally and a value set by one test would otherwise
// leak into the next.
func resetCommandFlags(t *testing.T) {
	t.Helper()

	previous := struct {
		global  bool
		verbose bool
		quiet   bool
		level   string
	}{globalScope, verbose, quiet, logLevel}

	globalScope = false
	verbose = false
	quiet = false
	logLevel = "info"

	addName, addPath, addRef = "", "", ""
	addType, addScope = "skill", "project"
	addTargets = nil

	installFrozen, installOffline = false, false
	syncOffline = false
	updateOffline, updateLatest = false, false
	listJSON = false
	verifyJSON = false
	outdatedJSON, outdatedOffline = false, false
	cleanCache, cleanAll = false, false
	pruneOlderThan, pruneMaxSize = "", ""
	pruneDryRun = false
	doctorJSON, treeJSON = false, false
	addDryRun, installDryRun, syncDryRun = false, false, false
	updateDryRun, removeDryRun = false, false

	t.Cleanup(func() {
		globalScope = previous.global
		verbose = previous.verbose
		quiet = previous.quiet
		logLevel = previous.level
	})
}

// resetConfigCache drops the parsed config file so a test that just pointed
// SKMGR_CONFIG somewhere new is actually read from there.
func resetConfigCache(t *testing.T) {
	t.Helper()

	config.Reset()
	t.Cleanup(config.Reset)
}

// writeManifest writes a manifest into the current directory.
func writeManifest(t *testing.T, filename string, contents string) {
	t.Helper()
	if err := os.WriteFile(filename, []byte(contents), 0644); err != nil {
		t.Fatalf("writing %s: %v", filename, err)
	}
}

// newSkillSource builds a local source tree holding the named skills and
// returns its path, ready to be used as a file:// source.
func newSkillSource(t *testing.T, names ...string) string {
	t.Helper()

	root := t.TempDir()
	for _, name := range names {
		dir := filepath.Join(root, "skills", name)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("creating skill source: %v", err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# "+name), 0644); err != nil {
			t.Fatalf("writing SKILL.md: %v", err)
		}
	}
	return root
}

// newTaggedSource builds a git repository holding one skill, with a commit and
// tag per version given. It returns a source string prefixed with git::, since
// a bare path would otherwise be read as a plain directory and refs would mean
// nothing.
func newTaggedSource(t *testing.T, skill string, versions ...string) string {
	t.Helper()

	root := t.TempDir()
	dir := filepath.Join(root, "skills", skill)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating skill source: %v", err)
	}

	gitInSource(t, root, "init", "--quiet", "-b", "main")

	for _, version := range versions {
		tagSourceVersion(t, root, skill, version)
	}

	return "git::" + root
}

// tagSourceVersion publishes one more release in a source built by
// newTaggedSource.
func tagSourceVersion(t *testing.T, root string, skill string, version string) {
	t.Helper()

	dir := filepath.Join(root, "skills", skill)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("creating skill source: %v", err)
	}

	contents := "# " + skill + "\n\nversion: " + version + "\n"
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(contents), 0644); err != nil {
		t.Fatalf("writing SKILL.md: %v", err)
	}

	gitInSource(t, root, "add", ".")
	gitInSource(t, root, "commit", "--quiet", "-m", "release "+version)
	gitInSource(t, root, "tag", version)
}

// installedSkillVersion reads the version marker out of an installed skill.
func installedSkillVersion(t *testing.T, projectRoot string, skill string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(projectRoot, ".agents", "skills", skill, "SKILL.md"))
	if err != nil {
		t.Fatalf("reading the installed skill: %v", err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if rest, ok := strings.CutPrefix(line, "version: "); ok {
			return strings.TrimSpace(rest)
		}
	}

	t.Fatalf("no version marker in the installed %s", skill)
	return ""
}

func gitInSource(t *testing.T, dir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// captureLog redirects the shared logger into a buffer for the duration of a
// test and returns it.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()

	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}
