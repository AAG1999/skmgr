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
	"slices"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var removeDryRun bool

var removeCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a skill/rule from the manifest and uninstall it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		proj, err := loadProject()
		if err != nil {
			return err
		}

		idx := slices.IndexFunc(proj.Manifest.Skills, func(s types.SkillDependency) bool {
			return s.Name == name
		})
		if idx == -1 {
			return fmt.Errorf("skill %q not found in manifest", name)
		}

		trimmed := *proj.Manifest
		trimmed.Skills = slices.Delete(slices.Clone(proj.Manifest.Skills), idx, idx+1)

		if removeDryRun {
			return runDryRun(cmd, proj, &trimmed, engine.SyncOptions{})
		}

		// Sync with the trimmed manifest first. Writing the manifest before the
		// sync succeeded would leave the skill gone from skmgr.yml while its
		// files, lock entry and agent links were all still in place.
		e := engine.NewEngine(proj.Root, types.CacheDir())

		newLock, err := e.Sync(&trimmed, proj.Lock, engine.SyncOptions{})
		if err != nil {
			return fmt.Errorf("failed to sync after removal: %w", err)
		}

		if err := manifest.Write(proj.ManifestPath, &trimmed); err != nil {
			return err
		}
		if err := lockfile.Write(proj.LockPath, newLock); err != nil {
			return fmt.Errorf("failed to save lockfile: %w", err)
		}

		log.Infof("Removed %q", name)
		return nil
	},
}

func init() {
	dryRunFlag(removeCmd, &removeDryRun, "the removal")
	rootCmd.AddCommand(removeCmd)
}
