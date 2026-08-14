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
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// releaseWithAssets builds a release listing naming the given asset files.
func releaseWithAssets(names ...string) *release {
	latest := &release{TagName: "v1.2.3", HTMLURL: "https://example.com/releases/v1.2.3"}
	for _, name := range names {
		latest.Assets = append(latest.Assets, struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		}{Name: name, URL: "https://example.com/" + name})
	}
	return latest
}

func TestAssetForPlatform_PicksTheMatchingBuild(t *testing.T) {
	mine := fmt.Sprintf("skmgr_1.2.3_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

	url, err := assetForPlatform(releaseWithAssets(
		"skmgr_1.2.3_someos_somearch.tar.gz",
		mine,
		"checksums.txt",
	))
	if err != nil {
		t.Fatalf("the build for this platform should have matched: %v", err)
	}

	if !strings.HasSuffix(url, mine) {
		t.Errorf("url = %q, want the asset for %s/%s", url, runtime.GOOS, runtime.GOARCH)
	}
}

// GoReleaser's default naming is title-cased with x86_64 rather than amd64, so
// matching only Go's own spelling would find nothing in a real release.
func TestAssetForPlatform_MatchesGoReleaserNaming(t *testing.T) {
	names := map[string]string{
		"linux/amd64":   "skmgr_Linux_x86_64.tar.gz",
		"linux/arm64":   "skmgr_Linux_arm64.tar.gz",
		"darwin/amd64":  "skmgr_Darwin_x86_64.tar.gz",
		"darwin/arm64":  "skmgr_Darwin_arm64.tar.gz",
		"windows/amd64": "skmgr_Windows_x86_64.zip",
	}

	want, ok := names[runtime.GOOS+"/"+runtime.GOARCH]
	if !ok {
		t.Skipf("no release naming fixture for %s/%s", runtime.GOOS, runtime.GOARCH)
	}

	// The full release listing, so the match has to discriminate rather than
	// take the first archive it sees.
	var all []string
	for _, name := range names {
		all = append(all, name)
	}
	all = append(all, "checksums.txt")

	url, err := assetForPlatform(releaseWithAssets(all...))
	if err != nil {
		t.Fatalf("a real release listing should have matched: %v", err)
	}
	if !strings.HasSuffix(url, want) {
		t.Errorf("url = %q, want the asset ending in %q", url, want)
	}
}

func TestMatchesPlatform_KnowsCommonAliases(t *testing.T) {
	cases := []struct {
		asset string
		value string
		want  bool
	}{
		{"skmgr_linux_x86_64.tar.gz", "amd64", true},
		{"skmgr_linux_amd64.tar.gz", "amd64", true},
		{"skmgr_darwin_aarch64.tar.gz", "arm64", true},
		{"skmgr_darwin_x86_64.tar.gz", "arm64", false},
		{"skmgr_macos_arm64.tar.gz", "darwin", true},
		{"skmgr_windows_x86_64.zip", "windows", true},
		{"skmgr_linux_x86_64.tar.gz", "windows", false},

		// "darwin" ends in "win", which a substring search reads as the Windows
		// alias and hands the macOS build to a Windows machine.
		{"skmgr_darwin_x86_64.tar.gz", "windows", false},
		{"skmgr_darwin_arm64.tar.gz", "windows", false},
	}

	for _, tc := range cases {
		if got := matchesPlatform(tc.asset, tc.value); got != tc.want {
			t.Errorf("matchesPlatform(%q, %q) = %v, want %v", tc.asset, tc.value, got, tc.want)
		}
	}
}

// A release with nothing for this platform has to say so, and point somewhere
// useful, rather than downloading a binary that will not run.
func TestAssetForPlatform_ExplainsAMissingBuild(t *testing.T) {
	_, err := assetForPlatform(releaseWithAssets("skmgr_1.2.3_plan9_mips.tar.gz", "checksums.txt"))
	if err == nil {
		t.Fatal("expected an error when there is no build for this platform")
	}

	for _, want := range []string{runtime.GOOS, "example.com"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the error should mention %q, got: %v", want, err)
		}
	}
}

// Checksums and release notes sit beside the archives and must not be mistaken
// for one.
func TestAssetForPlatform_IgnoresNonArchives(t *testing.T) {
	name := fmt.Sprintf("skmgr_1.2.3_%s_%s", runtime.GOOS, runtime.GOARCH)

	_, err := assetForPlatform(releaseWithAssets(name+".txt", name+".sbom.json"))
	if err == nil {
		t.Fatal("a text file is not an archive and should not be selected")
	}
}

func TestIsBinaryEntry(t *testing.T) {
	cases := map[string]bool{
		"skmgr":            true,
		"skmgr.exe":        true,
		"dist/skmgr":       true,
		"README.md":        false,
		"LICENSE":          false,
		"skmgr.completion": false,
	}

	for name, want := range cases {
		if got := isBinaryEntry(name); got != want {
			t.Errorf("isBinaryEntry(%q) = %v, want %v", name, got, want)
		}
	}
}

// writeTarGz builds a release-shaped tarball.
func writeTarGz(t *testing.T, entries map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "release.tar.gz")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	gz := gzip.NewWriter(file)
	tw := tar.NewWriter(gz)

	for name, contents := range entries {
		header := &tar.Header{
			Name:     name,
			Mode:     0755,
			Size:     int64(len(contents)),
			Typeflag: tar.TypeReg,
		}
		if err := tw.WriteHeader(header); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}

	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

// writeZip builds a release-shaped zip.
func writeZip(t *testing.T, entries map[string]string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "release.zip")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = file.Close() }()

	zw := zip.NewWriter(file)
	for name, contents := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(contents)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestExtractBinary_FromTarGz(t *testing.T) {
	archive := writeTarGz(t, map[string]string{
		"README.md": "not the binary",
		"skmgr":     "the binary",
	})

	dest := filepath.Join(t.TempDir(), "skmgr")
	if err := extractBinary(archive, dest); err != nil {
		t.Fatalf("extracting: %v", err)
	}

	contents, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "the binary" {
		t.Errorf("extracted %q, want the binary rather than a sibling file", contents)
	}
}

func TestExtractBinary_FromZip(t *testing.T) {
	archive := writeZip(t, map[string]string{
		"LICENSE":   "not the binary",
		"skmgr.exe": "the binary",
	})

	dest := filepath.Join(t.TempDir(), "skmgr.exe")
	if err := extractBinary(archive, dest); err != nil {
		t.Fatalf("extracting: %v", err)
	}

	contents, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != "the binary" {
		t.Errorf("extracted %q, want the binary", contents)
	}
}

// An archive that does not hold what we expect must fail loudly, since the
// alternative is overwriting a working skmgr with a README.
func TestExtractBinary_RejectsAnArchiveWithoutTheBinary(t *testing.T) {
	archive := writeTarGz(t, map[string]string{"README.md": "nothing useful"})

	err := extractBinary(archive, filepath.Join(t.TempDir(), "skmgr"))
	if err == nil {
		t.Fatal("expected an error when the archive holds no binary")
	}
	if !strings.Contains(err.Error(), "no skmgr binary") {
		t.Errorf("the error should say what was missing, got: %v", err)
	}
}

// Offline mode is a promise that nothing reaches the network, and self-update
// is no exception.
func TestSelfUpdate_RefusesWhenOffline(t *testing.T) {
	newProjectDir(t)
	t.Setenv("SKMGR_OFFLINE", "1")
	resetConfigCache(t)

	err := selfUpdateCmd.RunE(selfUpdateCmd, nil)
	if err == nil {
		t.Fatal("expected self-update to refuse in offline mode")
	}
	if !strings.Contains(err.Error(), "offline") {
		t.Errorf("the error should explain why, got: %v", err)
	}
}
