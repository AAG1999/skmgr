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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AAG1999/skmgr/internal/config"
	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/provider"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var doctorJSON bool

// checkState is the outcome of one diagnostic.
type checkState string

const (
	// checkPass means the environment is fine in this respect.
	checkPass checkState = "pass"

	// checkWarn means something is off but skmgr will still work, possibly in
	// a degraded way.
	checkWarn checkState = "warn"

	// checkFail means installs will not work correctly until it is fixed.
	checkFail checkState = "fail"
)

// checkResult is one line of the report, and also the JSON shape.
type checkResult struct {
	Name string `json:"name"`

	// State is pass, warn, or fail.
	State checkState `json:"state"`

	// Detail says what was found.
	Detail string `json:"detail"`

	// Remedy says what to do about it, and is empty when there is nothing
	// to do.
	Remedy string `json:"remedy,omitempty"`
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check that this machine and project are set up correctly",
	Long: `Diagnose the environment skmgr depends on.

doctor checks that git is available, that symlinks can be created, that the
canonical store is writable, that the cache is readable, and that installed
skills still match the lockfile. Nothing is modified.

The exit status is non-zero if any check fails, so it can gate a CI job.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		results := runChecks()

		if doctorJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(results); err != nil {
				return err
			}
		} else {
			printChecks(cmd.OutOrStdout(), results)
		}

		if failed := countState(results, checkFail); failed > 0 {
			// A plain error would print a second, redundant message under the
			// report that already explained each failure.
			return &silentError{code: fmt.Sprintf("%d check(s) failed", failed)}
		}

		return nil
	},
}

// runChecks gathers every diagnostic.
//
// Checks never stop at the first failure: someone running doctor wants the
// whole picture, not one problem at a time.
func runChecks() []checkResult {
	results := []checkResult{
		checkGit(),
		checkConfigFile(),
		checkCache(),
	}

	proj, err := loadProject()
	if err != nil {
		results = append(results, checkResult{
			Name:   "manifest",
			State:  checkWarn,
			Detail: err.Error(),
			Remedy: "run skmgr init to create a manifest",
		})
		// The remaining checks are all about a project, so there is nothing
		// further to say without one.
		return results
	}

	results = append(results,
		checkResult{
			Name:   "manifest",
			State:  checkPass,
			Detail: fmt.Sprintf("%s declares %d skills", proj.ManifestPath, len(proj.Manifest.Skills)),
		},
		checkCanonicalStore(proj),
		checkSymlinks(proj),
		checkLockfile(proj),
		checkInstalled(proj),
	)

	return results
}

// checkGit confirms git is on PATH, since every remote source needs it.
func checkGit() checkResult {
	path, err := exec.LookPath("git")
	if err != nil {
		return checkResult{
			Name:   "git",
			State:  checkFail,
			Detail: "git was not found on PATH",
			Remedy: "install git; skmgr cannot fetch remote skills without it",
		}
	}

	out, err := exec.Command("git", "--version").Output()
	if err != nil {
		return checkResult{
			Name:   "git",
			State:  checkFail,
			Detail: fmt.Sprintf("%s could not be run: %v", path, err),
			Remedy: "reinstall git",
		}
	}

	return checkResult{
		Name:   "git",
		State:  checkPass,
		Detail: strings.TrimSpace(string(out)),
	}
}

// checkConfigFile reports which config file is in effect, if any.
func checkConfigFile() checkResult {
	if err := config.Err(); err != nil {
		return checkResult{
			Name:   "config",
			State:  checkFail,
			Detail: err.Error(),
			Remedy: "fix the syntax, or delete the file to fall back to defaults",
		}
	}

	cfg := config.Current()
	if cfg.Path == "" {
		return checkResult{
			Name:   "config",
			State:  checkPass,
			Detail: fmt.Sprintf("no config file; using defaults (would read %s)", config.DefaultPath()),
		}
	}

	return checkResult{
		Name:   "config",
		State:  checkPass,
		Detail: fmt.Sprintf("reading %s", cfg.Path),
	}
}

// checkCache confirms the download cache is usable and reports its size.
func checkCache() checkResult {
	dir := types.CacheDir()

	if err := os.MkdirAll(dir, 0755); err != nil {
		return checkResult{
			Name:   "cache",
			State:  checkFail,
			Detail: fmt.Sprintf("%s cannot be created: %v", dir, err),
			Remedy: "check permissions, or set SKMGR_CACHE_DIR to a writable path",
		}
	}

	repos, err := provider.ScanRepos(dir)
	if err != nil {
		return checkResult{
			Name:   "cache",
			State:  checkWarn,
			Detail: fmt.Sprintf("%s could not be read: %v", dir, err),
			Remedy: "run skmgr clean --cache to start over",
		}
	}

	var bytes int64
	var commits int
	for _, repo := range repos {
		bytes += repo.CloneSize
		commits += len(repo.Commits)
		for _, commit := range repo.Commits {
			bytes += commit.Size
		}
	}

	return checkResult{
		Name:  "cache",
		State: checkPass,
		Detail: fmt.Sprintf("%s holds %d repositories and %d commits (%s)",
			dir, len(repos), commits, humanSize(bytes)),
	}
}

// checkCanonicalStore confirms skills can actually be written.
func checkCanonicalStore(proj *projectContext) checkResult {
	dir := types.CanonicalSkillsDir(types.ScopeProject, proj.Root)

	if err := os.MkdirAll(dir, 0755); err != nil {
		return checkResult{
			Name:   "store",
			State:  checkFail,
			Detail: fmt.Sprintf("%s cannot be created: %v", dir, err),
			Remedy: "check the permissions on the project directory",
		}
	}

	probe := filepath.Join(dir, ".skmgr-write-probe")
	if err := os.WriteFile(probe, []byte("probe"), 0644); err != nil {
		return checkResult{
			Name:   "store",
			State:  checkFail,
			Detail: fmt.Sprintf("%s is not writable: %v", dir, err),
			Remedy: "check the permissions on the project directory",
		}
	}
	_ = os.Remove(probe)

	return checkResult{
		Name:   "store",
		State:  checkPass,
		Detail: fmt.Sprintf("%s is writable", dir),
	}
}

// checkSymlinks tries to create one, because a filesystem or an unprivileged
// Windows session that refuses them changes how skills get installed.
func checkSymlinks(proj *projectContext) checkResult {
	dir, err := os.MkdirTemp(proj.Root, ".skmgr-symlink-probe-")
	if err != nil {
		return checkResult{
			Name:   "symlinks",
			State:  checkWarn,
			Detail: fmt.Sprintf("could not test symlinks: %v", err),
		}
	}
	defer func() { _ = os.RemoveAll(dir) }()

	target := filepath.Join(dir, "target")
	if err := os.Mkdir(target, 0755); err != nil {
		return checkResult{
			Name:   "symlinks",
			State:  checkWarn,
			Detail: fmt.Sprintf("could not test symlinks: %v", err),
		}
	}

	if err := os.Symlink("target", filepath.Join(dir, "link")); err != nil {
		return checkResult{
			Name:   "symlinks",
			State:  checkWarn,
			Detail: fmt.Sprintf("symlinks are not available: %v", err),
			Remedy: "skmgr will copy skills instead, and the copies will not " +
				"follow later updates; on Windows, enable Developer Mode",
		}
	}

	return checkResult{
		Name:   "symlinks",
		State:  checkPass,
		Detail: "symlinks can be created in this project",
	}
}

// checkLockfile reports whether the project is pinned.
func checkLockfile(proj *projectContext) checkResult {
	if proj.Lock == nil {
		return checkResult{
			Name:   "lockfile",
			State:  checkWarn,
			Detail: fmt.Sprintf("no %s yet", LockfileName),
			Remedy: "run skmgr install, then commit the lockfile so the team installs the same versions",
		}
	}

	return checkResult{
		Name:   "lockfile",
		State:  checkPass,
		Detail: fmt.Sprintf("%s pins %d skills", LockfileName, len(proj.Lock.Entries)),
	}
}

// checkInstalled compares what is on disk with what the lockfile records.
func checkInstalled(proj *projectContext) checkResult {
	if proj.Lock == nil {
		return checkResult{
			Name:   "installed",
			State:  checkWarn,
			Detail: "nothing to verify without a lockfile",
		}
	}

	e := engine.NewEngine(proj.Root, types.CacheDir())

	var missing, modified []string
	for _, result := range e.Verify(proj.Lock) {
		switch result.Status {
		case engine.VerifyMissing:
			missing = append(missing, result.Name)
		case engine.VerifyModified:
			modified = append(modified, result.Name)
		}
	}

	switch {
	case len(missing) > 0 && len(modified) > 0:
		return checkResult{
			Name:  "installed",
			State: checkFail,
			Detail: fmt.Sprintf("%d missing (%s), %d modified (%s)",
				len(missing), strings.Join(missing, ", "),
				len(modified), strings.Join(modified, ", ")),
			Remedy: "run skmgr sync",
		}

	case len(missing) > 0:
		return checkResult{
			Name:   "installed",
			State:  checkFail,
			Detail: fmt.Sprintf("%d skills are not installed: %s", len(missing), strings.Join(missing, ", ")),
			Remedy: "run skmgr install",
		}

	case len(modified) > 0:
		return checkResult{
			Name:   "installed",
			State:  checkFail,
			Detail: fmt.Sprintf("%d skills no longer match the lockfile: %s", len(modified), strings.Join(modified, ", ")),
			Remedy: "run skmgr sync to restore them, or update the lockfile if the change was intended",
		}

	default:
		return checkResult{
			Name:   "installed",
			State:  checkPass,
			Detail: fmt.Sprintf("all %d locked skills match", len(proj.Lock.Entries)),
		}
	}
}

// printChecks renders the report.
func printChecks(out io.Writer, results []checkResult) {
	for _, result := range results {
		_, _ = fmt.Fprintf(out, "%s %-10s %s\n", stateMarker(result.State), result.Name, result.Detail)
		if result.Remedy != "" {
			_, _ = fmt.Fprintf(out, "             -> %s\n", result.Remedy)
		}
	}

	failed := countState(results, checkFail)
	warned := countState(results, checkWarn)

	_, _ = fmt.Fprintln(out)
	switch {
	case failed > 0:
		_, _ = fmt.Fprintf(out, "%d failed, %d warnings, %d passed\n",
			failed, warned, countState(results, checkPass))
	case warned > 0:
		_, _ = fmt.Fprintf(out, "No failures, %d warnings\n", warned)
	default:
		_, _ = fmt.Fprintln(out, "Everything looks healthy")
	}
}

// stateMarker is the leading symbol for a result line.
func stateMarker(state checkState) string {
	switch state {
	case checkPass:
		return "[ ok ]"
	case checkWarn:
		return "[warn]"
	default:
		return "[fail]"
	}
}

// countState counts results in one state.
func countState(results []checkResult, state checkState) int {
	var count int
	for _, result := range results {
		if result.State == state {
			count++
		}
	}
	return count
}

func init() {
	doctorCmd.Flags().BoolVar(&doctorJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(doctorCmd)
}
