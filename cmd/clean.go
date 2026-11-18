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
	"fmt"
	"os"
	"path/filepath"

	"github.com/AAG1999/skmgr/internal/linker"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	cleanCache bool
	cleanAll   bool
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove installed skills, managed rule blocks, and/or the cache",
	Long: `Remove what skmgr installed.

By default this clears the project: .agents/ is deleted, broken agent symlinks
are pruned, and skmgr-managed blocks are stripped from merged rule files such as
CLAUDE.md. Your own content in those files is preserved.

With --global it clears ~/.agents/ and the agent directories under your home
directory instead, and releases the global ownership ledger.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}

		if cleanAll {
			cleanCache = true
		}

		if cleanCache {
			cacheDir := types.CacheDir()
			if err := os.RemoveAll(cacheDir); err != nil {
				return fmt.Errorf("failed to clear cache: %w", err)
			}
			log.Infof("Cache cleaned (%s)", cacheDir)
		}

		// --cache on its own means "only the cache". --all means everything.
		if cleanCache && !cleanAll {
			if jsonOutput {
				return emit(cmd, map[string]bool{"cleaned": true})
			}
			return nil
		}

		scope := types.ScopeProject
		if globalScope {
			scope = types.ScopeGlobal
		}

		if err := cleanScope(scope, cwd); err != nil {
			return err
		}

		if cleanAll && !globalScope {
			// --all is "everything skmgr put on this machine", which includes
			// the shared global tree.
			if err := cleanScope(types.ScopeGlobal, cwd); err != nil {
				return err
			}
		}

		if jsonOutput {
			return emit(cmd, map[string]bool{"cleaned": true})
		}
		return nil
	},
}

// cleanScope removes the canonical store for a scope and undoes every
// projection of it: broken symlinks and managed blocks in merged rule files.
func cleanScope(scope string, projectRoot string) error {
	canonical := types.CanonicalDir(scope, projectRoot)

	l := linker.NewLinker()

	// Strip merged blocks before deleting the canonical store, since the
	// managed sections are the only record that those rules were installed.
	if err := l.StripManagedRules(scope, projectRoot); err != nil {
		return err
	}

	if err := removeCanonicalChildren(canonical); err != nil {
		return fmt.Errorf("failed to remove %s: %w", canonical, err)
	}

	if err := l.CleanBrokenLinks(scope, projectRoot); err != nil {
		return fmt.Errorf("failed to clean broken links: %w", err)
	}

	if scope == types.ScopeGlobal {
		// The tree is gone, so no project owns anything in it any more. Leaving
		// stale ownership behind would make the next install think other
		// projects still need skills that no longer exist.
		if err := releaseGlobalLedger(projectRoot); err != nil {
			return err
		}
	}

	log.Infof("Cleaned %s scope (%s removed, managed rule blocks stripped, broken symlinks pruned)", scope, canonical)
	return nil
}

// releaseGlobalLedger drops this project's claim and clears adopted entries,
// since an explicit global clean is the user deliberately resetting the tree.
func releaseGlobalLedger(projectRoot string) error {
	ledgerPath := lockfile.GlobalLedgerPath(types.SkmgrHome())

	ledger, err := lockfile.ReadGlobalLedger(ledgerPath)
	if err != nil {
		return fmt.Errorf("reading global ledger: %w", err)
	}

	ledger.SetProject(projectRoot, nil)
	ledger.ReleaseAdopted()

	if err := lockfile.WriteGlobalLedger(ledgerPath, ledger); err != nil {
		return fmt.Errorf("updating global ledger: %w", err)
	}

	return nil
}

// removeCanonicalChildren deletes installed content while leaving the
// generated workflow skills in place. Those files are real files inside
// .agents/skills for Gemini, and a blanket RemoveAll would take them with it.
func removeCanonicalChildren(canonical string) error {
	entries, err := os.ReadDir(canonical)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(canonical, entry.Name())
		if entry.IsDir() && entry.Name() == "skills" {
			if err := removeExceptReserved(path); err != nil {
				return err
			}
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			return err
		}
	}
	return nil
}

func removeExceptReserved(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if types.IsReservedWorkflowName(entry.Name()) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanCache, "cache", false, "Clean the global download cache instead of the project")
	cleanCmd.Flags().BoolVar(&cleanAll, "all", false, "Clean the project, the global scope, and the cache")
	addGlobalFlag(cleanCmd, "Clean the global scope (~/.agents/) instead of the project")
	jsonFlag(cleanCmd)
	rootCmd.AddCommand(cleanCmd)
}
