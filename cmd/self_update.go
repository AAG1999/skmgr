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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/AAG1999/skmgr/internal/log"
	"github.com/spf13/cobra"
)

var (
	selfUpdateCheck bool
	selfUpdateForce bool
)

// releaseAPI is the endpoint listing the newest published release.
const releaseAPI = "https://api.github.com/repos/AAG1999/skmgr/releases/latest"

// downloadTimeout bounds the whole exchange, so a hung mirror does not leave
// the command waiting forever.
const downloadTimeout = 2 * time.Minute

// release is the part of the GitHub API response we use.
type release struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

var selfUpdateCmd = &cobra.Command{
	Use:   "self-update",
	Short: "Update skmgr to the latest release",
	Long: `Download the newest release from GitHub and replace this binary.

The new binary is written next to the current one and swapped in atomically, so
an interrupted download cannot leave you without a working skmgr.

Use --check to see whether an update exists without installing it. If skmgr came
from a package manager, update it there instead so the two do not disagree.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if offlineRequested(false) {
			return fmt.Errorf("self-update needs network access, but offline mode is on")
		}

		latest, err := fetchLatestRelease(cmd.Context())
		if err != nil {
			return err
		}

		current := strings.TrimPrefix(Version, "v")
		newest := strings.TrimPrefix(latest.TagName, "v")

		if current == newest && !selfUpdateForce {
			log.Infof("Already on the latest release (%s)", latest.TagName)
			return nil
		}

		if selfUpdateCheck {
			log.Infof("A newer release is available: %s -> %s", Version, latest.TagName)
			log.Infof("Release notes: %s", latest.HTMLURL)
			log.Infof("Run 'skmgr self-update' to install it")
			return nil
		}

		asset, err := assetForPlatform(latest)
		if err != nil {
			return err
		}

		log.Infof("Downloading %s...", asset)
		if err := replaceBinary(cmd.Context(), asset); err != nil {
			return err
		}

		log.Infof("Updated skmgr %s -> %s", Version, latest.TagName)
		return nil
	},
}

// fetchLatestRelease asks GitHub what the newest release is.
func fetchLatestRelease(ctx context.Context) (*release, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("checking for a new release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("no published releases yet")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("checking for a new release: GitHub returned %s", resp.Status)
	}

	var latest release
	if err := json.NewDecoder(resp.Body).Decode(&latest); err != nil {
		return nil, fmt.Errorf("reading the release listing: %w", err)
	}
	if latest.TagName == "" {
		return nil, fmt.Errorf("the release listing had no version in it")
	}

	return &latest, nil
}

// platformAliases are the other spellings release archives use for the same
// operating system or architecture. GoReleaser's default naming produces
// "skmgr_Linux_x86_64.tar.gz", so matching on Go's own "linux_amd64" alone
// would never find the build for this machine.
var platformAliases = map[string][]string{
	"amd64":   {"amd64", "x86_64", "x64"},
	"arm64":   {"arm64", "aarch64"},
	"386":     {"386", "i386", "x86"},
	"darwin":  {"darwin", "macos", "osx", "mac"},
	"windows": {"windows", "win"},
	"linux":   {"linux"},
}

// assetForPlatform picks the archive matching this machine.
func assetForPlatform(latest *release) (string, error) {
	for _, asset := range latest.Assets {
		name := strings.ToLower(asset.Name)

		if isArchive(name) && matchesPlatform(name, runtime.GOOS) && matchesPlatform(name, runtime.GOARCH) {
			return asset.URL, nil
		}
	}

	return "", fmt.Errorf(
		"release %s has no build for %s/%s; install it manually from %s",
		latest.TagName, runtime.GOOS, runtime.GOARCH, latest.HTMLURL,
	)
}

// matchesPlatform reports whether an asset name names this OS or architecture,
// under any of the spellings in common use.
func matchesPlatform(assetName string, value string) bool {
	names, ok := platformAliases[value]
	if !ok {
		names = []string{value}
	}

	for _, name := range names {
		if containsWord(assetName, name) {
			return true
		}
	}
	return false
}

// containsWord reports whether name contains word delimited by separators
// rather than buried inside a longer run of letters and digits.
//
// A plain substring search is wrong here: "win" is a real alias for Windows and
// also the last three letters of "darwin", so `skmgr self-update` on Windows
// downloaded the macOS build. Separators still count as boundaries so that a
// compound spelling like "x86_64" matches.
func containsWord(name string, word string) bool {
	for start := 0; start <= len(name)-len(word); {
		index := strings.Index(name[start:], word)
		if index < 0 {
			return false
		}
		index += start

		if isWordBoundary(name, index-1) && isWordBoundary(name, index+len(word)) {
			return true
		}
		start = index + 1
	}
	return false
}

// isWordBoundary reports whether the byte at index ends a word. Positions
// outside the string are boundaries, as is any byte that is not alphanumeric.
func isWordBoundary(name string, index int) bool {
	if index < 0 || index >= len(name) {
		return true
	}

	switch c := name[index]; {
	case 'a' <= c && c <= 'z', 'A' <= c && c <= 'Z', '0' <= c && c <= '9':
		return false
	default:
		return true
	}
}

// isArchive reports whether a release asset is one we know how to unpack.
func isArchive(name string) bool {
	return strings.HasSuffix(name, ".tar.gz") || strings.HasSuffix(name, ".zip")
}

// replaceBinary downloads an archive and swaps in the binary it contains.
//
// The replacement is staged beside the current binary and moved into place with
// a rename, which is atomic on every supported platform. A partial download can
// therefore never be left as the installed skmgr.
func replaceBinary(ctx context.Context, url string) error {
	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating the current binary: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("resolving the current binary: %w", err)
	}

	archive, err := download(ctx, url)
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(archive) }()

	staged := self + ".new"
	if err := extractBinary(archive, staged); err != nil {
		_ = os.Remove(staged)
		return err
	}

	if err := os.Chmod(staged, 0755); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("making the new binary executable: %w", err)
	}

	// Windows refuses to replace a running executable, so the old one is moved
	// aside first and cleaned up on the next run.
	if runtime.GOOS == "windows" {
		old := self + ".old"
		_ = os.Remove(old)
		if err := os.Rename(self, old); err != nil {
			_ = os.Remove(staged)
			return fmt.Errorf("moving the old binary aside: %w", err)
		}
	}

	if err := os.Rename(staged, self); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("installing the new binary at %s: %w "+
			"(if skmgr is installed system-wide, run this with elevated permissions "+
			"or update through your package manager)", self, err)
	}

	return nil
}

// download fetches a URL to a temporary file.
func download(ctx context.Context, url string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("downloading the release: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading the release: server returned %s", resp.Status)
	}

	file, err := os.CreateTemp("", "skmgr-update-*")
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()

	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = os.Remove(file.Name())
		return "", fmt.Errorf("downloading the release: %w", err)
	}

	return file.Name(), nil
}

// extractBinary pulls the skmgr executable out of a release archive.
func extractBinary(archive string, dest string) error {
	if strings.HasSuffix(archive, ".zip") || isZip(archive) {
		return extractFromZip(archive, dest)
	}
	return extractFromTarGz(archive, dest)
}

// isZip sniffs the magic bytes, since the temporary file has no extension.
func isZip(path string) bool {
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = file.Close() }()

	magic := make([]byte, 2)
	if _, err := io.ReadFull(file, magic); err != nil {
		return false
	}
	return magic[0] == 'P' && magic[1] == 'K'
}

// extractFromTarGz writes the skmgr entry of a tarball to dest.
func extractFromTarGz(archive string, dest string) error {
	file, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("reading the release archive: %w", err)
	}
	defer func() { _ = gz.Close() }()

	reader := tar.NewReader(gz)
	for {
		header, err := reader.Next()
		if err == io.EOF {
			return fmt.Errorf("the release archive contained no skmgr binary")
		}
		if err != nil {
			return fmt.Errorf("reading the release archive: %w", err)
		}

		if header.Typeflag != tar.TypeReg || !isBinaryEntry(header.Name) {
			continue
		}
		return writeFile(dest, reader)
	}
}

// extractFromZip writes the skmgr entry of a zip to dest.
func extractFromZip(archive string, dest string) error {
	reader, err := zip.OpenReader(archive)
	if err != nil {
		return fmt.Errorf("reading the release archive: %w", err)
	}
	defer func() { _ = reader.Close() }()

	for _, entry := range reader.File {
		if entry.FileInfo().IsDir() || !isBinaryEntry(entry.Name) {
			continue
		}

		file, err := entry.Open()
		if err != nil {
			return fmt.Errorf("reading the release archive: %w", err)
		}

		err = writeFile(dest, file)
		_ = file.Close()
		return err
	}

	return fmt.Errorf("the release archive contained no skmgr binary")
}

// isBinaryEntry reports whether an archive entry is the skmgr executable,
// ignoring the README and licence that ship alongside it.
func isBinaryEntry(name string) bool {
	base := filepath.Base(filepath.FromSlash(name))
	return base == "skmgr" || base == "skmgr.exe"
}

// writeFile streams a reader into a new file.
func writeFile(dest string, src io.Reader) error {
	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("writing the new binary: %w", err)
	}
	defer func() { _ = out.Close() }()

	if _, err := io.Copy(out, src); err != nil {
		return fmt.Errorf("writing the new binary: %w", err)
	}
	return out.Sync()
}

func init() {
	selfUpdateCmd.Flags().BoolVar(&selfUpdateCheck, "check", false,
		"Report whether a newer release exists, without installing it")
	selfUpdateCmd.Flags().BoolVar(&selfUpdateForce, "force", false,
		"Reinstall even if the current version is already the latest")
	rootCmd.AddCommand(selfUpdateCmd)
}
