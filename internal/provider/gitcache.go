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
	"archive/tar"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/semver"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/AAG1999/skmgr/internal/util/lock"
)

// cacheLockTimeout bounds how long we wait for another skmgr process to finish
// with the same repository. Fetches are short; a wait longer than this almost
// certainly means a previous run died holding the lock.
const cacheLockTimeout = 5 * time.Minute

// gitCache is the on-disk layout for one source repository.
//
//	<cacheDir>/git/<source-hash>/
//	    .lock           guards clone and fetch
//	    repo/           the clone, checked out to nothing
//	    commits/<sha>/  one extracted tree per commit
//
// Commit directories are immutable once written, which is what makes the cache
// safe to share. The previous layout checked the requested ref out inside a
// single shared clone, so two runs wanting different refs of the same
// repository would each see the other's checkout.
type gitCache struct {
	root    string
	repoDir string
	lockDir string
}

func newGitCache(cacheDir string, source string) gitCache {
	dropLegacyCache(cacheDir)

	sum := sha256.Sum256([]byte(source))
	root := filepath.Join(cacheDir, "git", hex.EncodeToString(sum[:]))

	return gitCache{
		root:    root,
		repoDir: filepath.Join(root, "repo"),
		lockDir: filepath.Join(root, ".lock"),
	}
}

// commitDir returns the extracted tree for a commit.
func (c gitCache) commitDir(sha string) string {
	return filepath.Join(c.root, "commits", sha)
}

// cloned reports whether the repository has been cloned already.
func (c gitCache) cloned() bool {
	_, err := os.Stat(filepath.Join(c.repoDir, "HEAD"))
	if err == nil {
		return true
	}
	// A non-bare clone keeps its metadata in .git.
	_, err = os.Stat(filepath.Join(c.repoDir, ".git"))
	return err == nil
}

// materializeResult is what a cache lookup produces.
type materializeResult struct {
	// Dir is the immutable tree for the resolved commit.
	Dir string

	// SHA is that commit.
	SHA string

	// ResolvedRef is the tag a version range selected, empty for literal refs.
	ResolvedRef string
}

// materialize returns a directory holding the tree at the requested ref,
// fetching and extracting it if necessary.
//
// Everything that touches shared state happens under the lock; the returned
// directory is immutable, so callers may read it after the lock is released.
func (c gitCache) materialize(source string, ref string, opts FetchOptions) (materializeResult, error) {
	var result materializeResult

	if err := os.MkdirAll(c.root, 0755); err != nil {
		return result, fmt.Errorf("creating cache directory: %w", err)
	}

	l, err := lock.Acquire(c.lockDir, cacheLockTimeout)
	if err != nil {
		return result, err
	}
	defer func() { _ = l.Unlock() }()

	if err := c.ensureRepo(source, opts); err != nil {
		return result, err
	}

	// A version range names a set of tags rather than one ref, so it has to be
	// narrowed to a concrete tag before anything can be checked out.
	if semver.IsRange(ref) || opts.Latest {
		tag, err := resolveRange(c.repoDir, ref, opts.Latest)
		if err != nil {
			return result, fmt.Errorf("resolving %q against %s: %w", ref, source, err)
		}
		log.Debugf("%s resolved %q to tag %s", source, ref, tag)
		result.ResolvedRef = tag
		ref = tag
	}

	target, err := resolveCheckoutTarget(c.repoDir, ref, opts)
	if err != nil {
		return result, err
	}

	sha, err := revSHA(c.repoDir, target)
	if err != nil {
		return result, err
	}
	result.SHA = sha

	result.Dir = c.commitDir(sha)
	if _, err := os.Stat(result.Dir); err == nil {
		log.Debugf("cache hit for %s at %s", source, sha[:min(len(sha), 12)])
		return result, nil
	}

	if err := c.extract(sha, result.Dir); err != nil {
		return materializeResult{}, err
	}

	return result, nil
}

// withRepo makes sure the clone exists and is current, for callers that want to
// inspect the repository without materializing a tree.
func (c gitCache) withRepo(source string, opts FetchOptions) error {
	if err := os.MkdirAll(c.root, 0755); err != nil {
		return fmt.Errorf("creating cache directory: %w", err)
	}

	return lock.WithLock(c.lockDir, cacheLockTimeout, func() error {
		return c.ensureRepo(source, opts)
	})
}

// ensureRepo clones the repository if needed, and otherwise refreshes it.
func (c gitCache) ensureRepo(source string, opts FetchOptions) error {
	if !c.cloned() {
		if opts.Offline {
			return fmt.Errorf(
				"offline: %q is not in the cache; run without --offline once to populate it", source)
		}
		return c.clone(source)
	}

	if opts.Offline {
		// Whatever the cache holds is all we get.
		return nil
	}

	return c.fetch(source)
}

// clone creates the local copy.
//
// The clone is deliberately complete rather than shallow: a shallow clone only
// carries the default branch, so any manifest naming a tag or another branch
// had to re-fetch on every run, and some refs could not be resolved at all.
// --no-checkout keeps it cheap, since trees are extracted per commit anyway.
func (c gitCache) clone(source string) error {
	if err := types.ValidateGitArgument("source", source); err != nil {
		return err
	}

	if err := os.RemoveAll(c.repoDir); err != nil {
		return fmt.Errorf("clearing partial clone: %w", err)
	}

	return runGitWithRetry("clone", func() *exec.Cmd {
		// The trailing -- ends option parsing, so a source that somehow reached
		// here could still not be read as a flag.
		return exec.Command("git", "clone", "--quiet", "--no-checkout", "--", source, c.repoDir)
	})
}

// fetch refreshes remote-tracking refs and tags so moving refs advance.
func (c gitCache) fetch(source string) error {
	err := runGitWithRetry("fetch", func() *exec.Cmd {
		cmd := exec.Command("git", "fetch", "--quiet", "--prune", "--tags", "origin")
		cmd.Dir = c.repoDir
		return cmd
	})
	if err != nil {
		return fmt.Errorf("fetching %q: %w", source, err)
	}
	return nil
}

// extract writes the tree at sha into dir.
//
// git archive is used rather than a checkout because it cannot disturb the
// shared clone and produces a plain tree with no git metadata to strip later.
// The stream is unpacked by untar rather than the tar binary. The extraction
// lands in a staging directory first so an interrupted run cannot leave a
// partial tree that later looks like a cache hit.
func (c gitCache) extract(sha string, dir string) error {
	if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
		return fmt.Errorf("creating commit cache directory: %w", err)
	}

	staging, err := os.MkdirTemp(filepath.Dir(dir), "."+sha+".tmp-*")
	if err != nil {
		return fmt.Errorf("creating staging directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(staging) }()

	archive := exec.Command("git", "archive", "--format=tar", sha)
	archive.Dir = c.repoDir

	pipe, err := archive.StdoutPipe()
	if err != nil {
		return fmt.Errorf("preparing archive of %s: %w", sha, err)
	}

	var archiveErr strings.Builder
	archive.Stderr = &archiveErr

	if err := archive.Start(); err != nil {
		return fmt.Errorf("archiving %s: %w", sha, err)
	}

	if err := untar(pipe, staging); err != nil {
		_ = archive.Process.Kill()
		_ = archive.Wait()
		return fmt.Errorf("extracting %s: %w", sha, err)
	}

	// tar.Reader stops at the end-of-archive marker, but git archive pads its
	// output to a whole record, so up to a few kilobytes are still in flight.
	// Waiting without reading them deadlocks: git blocks writing into a full
	// pipe and never exits. Linux pipe buffers are large enough to hide this,
	// Windows ones are not.
	if _, err := io.Copy(io.Discard, pipe); err != nil {
		_ = archive.Process.Kill()
		_ = archive.Wait()
		return fmt.Errorf("reading the rest of the archive of %s: %w", sha, err)
	}

	if err := archive.Wait(); err != nil {
		return fmt.Errorf("archiving %s: %v\n%s", sha, err, archiveErr.String())
	}

	if err := os.Rename(staging, dir); err != nil {
		// Another process may have won the race and published it first, which
		// is fine: the content is identical either way.
		if _, statErr := os.Stat(dir); statErr == nil {
			return nil
		}
		return fmt.Errorf("publishing cache entry for %s: %w", sha, err)
	}

	return nil
}

// untar writes a tar stream into dir.
//
// The stream is decoded in process rather than piped to the tar binary. Git for
// Windows ships an MSYS tar that cannot open a drive-letter path, so `tar -C
// C:\Users\...` fails on every Windows machine; decoding here also drops a
// runtime dependency on a tar that POSIX does not require to exist.
//
// git archive emits only directories, regular files and symlinks, so anything
// else in the stream is skipped rather than guessed at.
func untar(r io.Reader, dir string) error {
	reader := tar.NewReader(r)

	for {
		header, err := reader.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading the archive: %w", err)
		}

		// A repository is remote content, so an entry named "../../.ssh" must
		// not be able to write outside the staging directory.
		target, err := resolveWithinBase(dir, header.Name)
		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("creating %q: %w", header.Name, err)
			}

		case tar.TypeReg:
			if err := writeArchivedFile(target, reader, header.FileInfo().Mode().Perm()); err != nil {
				return fmt.Errorf("writing %q: %w", header.Name, err)
			}

		case tar.TypeSymlink:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return fmt.Errorf("creating the parent of %q: %w", header.Name, err)
			}
			if err := os.Symlink(filepath.FromSlash(header.Linkname), target); err != nil {
				return fmt.Errorf("linking %q: %w", header.Name, err)
			}
		}
	}
}

// writeArchivedFile creates one file from the archive stream.
func writeArchivedFile(path string, r io.Reader, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return err
	}

	if _, err := io.Copy(file, r); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// revSHA resolves a revision to a full commit SHA.
func revSHA(repoDir string, rev string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", rev+"^{commit}")
	cmd.Dir = repoDir

	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("resolving %q to a commit: %w", rev, err)
	}

	sha := strings.TrimSpace(string(out))
	if sha == "" {
		return "", fmt.Errorf("resolving %q produced no commit", rev)
	}
	return sha, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
