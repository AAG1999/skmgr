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
)

// legacyName is a plausible old cache directory: 64 hex characters.
const legacyName = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func TestDropLegacyCache_RemovesOldPerSourceClones(t *testing.T) {
	cacheDir := t.TempDir()

	legacy := filepath.Join(cacheDir, legacyName)
	if err := os.MkdirAll(filepath.Join(legacy, ".git"), 0755); err != nil {
		t.Fatal(err)
	}

	newGitCache(cacheDir, "https://example.com/repo.git")

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("the outdated cache entry survived: %v", err)
	}
}

func TestDropLegacyCache_KeepsTheCurrentLayout(t *testing.T) {
	cacheDir := t.TempDir()

	commit := filepath.Join(cacheDir, "git", legacyName, "commits", "abc123")
	if err := os.MkdirAll(commit, 0755); err != nil {
		t.Fatal(err)
	}

	newGitCache(cacheDir, "https://example.com/repo.git")

	if _, err := os.Stat(commit); err != nil {
		t.Errorf("a current cache entry was removed: %v", err)
	}
}

func TestDropLegacyCache_LeavesUnrelatedNamesAlone(t *testing.T) {
	cacheDir := t.TempDir()

	// Neither the right length nor all hex, so neither can be an old entry.
	keep := []string{"git", "notes.txt", strings.Repeat("z", 64), "abc123"}

	for _, name := range keep {
		if err := os.MkdirAll(filepath.Join(cacheDir, name), 0755); err != nil {
			t.Fatal(err)
		}
	}

	newGitCache(cacheDir, "https://example.com/repo.git")

	for _, name := range keep {
		if _, err := os.Stat(filepath.Join(cacheDir, name)); err != nil {
			t.Errorf("%q was removed but is not an outdated cache entry: %v", name, err)
		}
	}
}

func TestIsLegacyCacheDirName(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{name: legacyName, want: true},
		{name: strings.ToUpper(legacyName), want: false},
		{name: legacyName[:63], want: false},
		{name: legacyName + "0", want: false},
		{name: "git", want: false},
		{name: "", want: false},
	}

	for _, tc := range cases {
		if got := isLegacyCacheDirName(tc.name); got != tc.want {
			t.Errorf("isLegacyCacheDirName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
