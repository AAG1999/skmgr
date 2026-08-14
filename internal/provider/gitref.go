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
	"fmt"
	"os/exec"
	"strings"

	"github.com/AAG1999/skmgr/internal/types"
)

// resolveCheckoutTarget turns a user-supplied ref into a revision that is safe
// to check out detached.
//
// The important case is a branch. A bare branch name resolves to the *local*
// branch created at clone time, which `git fetch` never fast-forwards, so the
// cache would serve the same stale commit forever. Branches are therefore
// resolved to their `origin/<ref>` counterpart, which always reflects what was
// just fetched. Tags and commit SHAs are immutable and are used as-is.
func resolveCheckoutTarget(repoDir string, requestedRef string, opts FetchOptions) (string, error) {
	ref := strings.TrimSpace(requestedRef)

	if ref == "" {
		return defaultBranchRef(repoDir)
	}

	// A ref reaches git as a bare argument, and `--` cannot protect it because
	// for checkout that separator introduces paths rather than ending options.
	if err := types.ValidateGitArgument("ref", ref); err != nil {
		return "", err
	}

	// An explicit origin/... ref is already remote-tracking.
	if strings.HasPrefix(ref, "origin/") {
		if revExists(repoDir, ref) {
			return ref, nil
		}
	}

	// Prefer the remote-tracking branch so moving refs stay current.
	if remote := "refs/remotes/origin/" + ref; revExists(repoDir, remote) {
		return remote, nil
	}

	// Tags and SHAs are immutable; use them directly.
	if tag := "refs/tags/" + ref; revExists(repoDir, tag) {
		return tag, nil
	}
	if revExists(repoDir, ref) {
		return ref, nil
	}

	if opts.Offline {
		return "", fmt.Errorf("offline: ref %q is not present in the cache", ref)
	}

	// A shallow clone only carries the default branch, so a ref that exists on
	// the remote may simply not be local yet. Fetch it explicitly and retry.
	if err := fetchSpecificRef(repoDir, ref); err == nil {
		if revExists(repoDir, "refs/remotes/origin/"+ref) {
			return "refs/remotes/origin/" + ref, nil
		}
		if revExists(repoDir, "refs/tags/"+ref) {
			return "refs/tags/" + ref, nil
		}
		if revExists(repoDir, "FETCH_HEAD") {
			return "FETCH_HEAD", nil
		}
	}

	return "", fmt.Errorf("ref %q not found in repository (not a branch, tag, or commit)", ref)
}

// defaultBranchRef returns the remote's default branch, falling back to the
// common names when the remote HEAD symbolic ref is unavailable.
func defaultBranchRef(repoDir string) (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoDir
	if out, err := cmd.Output(); err == nil {
		if ref := strings.TrimSpace(string(out)); ref != "" && revExists(repoDir, ref) {
			return ref, nil
		}
	}

	for _, candidate := range []string{
		"refs/remotes/origin/main",
		"refs/remotes/origin/master",
		"refs/remotes/origin/trunk",
	} {
		if revExists(repoDir, candidate) {
			return candidate, nil
		}
	}

	// Single-branch shallow clones sometimes leave no remote-tracking ref at
	// all; the local HEAD is then the only thing pointing at the fetched tip.
	if revExists(repoDir, "HEAD") {
		return "HEAD", nil
	}

	return "", fmt.Errorf("could not determine the default branch of the cached repository")
}

// revExists reports whether a revision resolves in the repository.
func revExists(repoDir string, rev string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	cmd.Dir = repoDir
	return cmd.Run() == nil
}

// fetchSpecificRef pulls one ref into the cache. Used when a shallow clone did
// not bring down the branch or tag the manifest asks for.
func fetchSpecificRef(repoDir string, ref string) error {
	refspec := fmt.Sprintf("+%s:refs/remotes/origin/%s", ref, ref)

	cmd := exec.Command("git", "fetch", "--quiet", "--tags", "origin", refspec)
	cmd.Dir = repoDir
	if err := cmd.Run(); err == nil {
		return nil
	}

	// The ref may be a bare commit SHA, which cannot be used in that refspec.
	cmd = exec.Command("git", "fetch", "--quiet", "origin", ref)
	cmd.Dir = repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git fetch failed for ref %q: %v\nOutput: %s", ref, err, string(out))
	}

	return nil
}
