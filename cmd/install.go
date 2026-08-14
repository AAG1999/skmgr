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
	"errors"
	"fmt"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	installFrozen  bool
	installOffline bool
	installDryRun  bool
)

var installCmd = &cobra.Command{
	Use:   "install",
	Short: "Install all skills from the manifest",
	Long: `Install every skill declared in the manifest.

With --frozen, skills are pinned to the commit SHAs recorded in skmgr.lock and
no remotes are contacted. Use it in CI so a moving branch cannot change what
gets installed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		opts := engine.SyncOptions{
			Frozen:  installFrozen,
			Offline: offlineRequested(installOffline),
		}

		if installDryRun {
			return runDryRun(cmd, proj, proj.Manifest, opts)
		}

		e := engine.NewEngine(proj.Root, types.CacheDir())

		log.Debugf("installing %d declared skills from %s", len(proj.Manifest.Skills), proj.ManifestPath)
		if installFrozen {
			log.Debugf("frozen mode: pinning to %s and skipping remote resolution", LockfileName)
		}

		newLock, err := e.Sync(proj.Manifest, proj.Lock, opts)
		if err != nil {
			// A lockfile mismatch is already a complete, actionable message.
			var mismatch *engine.HashMismatchError
			if errors.As(err, &mismatch) {
				return err
			}
			return fmt.Errorf("installation failed: %w", err)
		}

		if err := lockfile.Write(proj.LockPath, newLock); err != nil {
			return fmt.Errorf("writing lockfile: %w", err)
		}

		log.Infof("Installed %d skills", len(newLock.Entries))
		return nil
	},
}

func init() {
	installCmd.Flags().BoolVar(&installFrozen, "frozen", false,
		"Install exactly what skmgr.lock records; do not contact remotes")
	installCmd.Flags().BoolVar(&installOffline, "offline", false,
		"Never access the network; use only the local cache (or set SKMGR_OFFLINE=1)")
	dryRunFlag(installCmd, &installDryRun, "the install")
	rootCmd.AddCommand(installCmd)
}
