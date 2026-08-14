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

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	syncOffline bool
	syncDryRun  bool
)

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Align installed state with the manifest (alias for install)",
	Long: `Bring .agents/ and the agent directories back in line with the manifest.

sync runs the same pipeline as install: it reinstalls declared skills, removes
orphans, and prunes broken links. Use it after editing skmgr.yml by hand or
after files under .agents/ have been disturbed.

For a pinned, reproducible install use "skmgr install --frozen".`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		opts := engine.SyncOptions{Offline: offlineRequested(syncOffline)}

		if syncDryRun {
			return runDryRun(cmd, proj, proj.Manifest, opts)
		}

		e := engine.NewEngine(proj.Root, types.CacheDir())

		newLock, err := e.Sync(proj.Manifest, proj.Lock, opts)
		if err != nil {
			return fmt.Errorf("sync failed: %w", err)
		}

		if err := lockfile.Write(proj.LockPath, newLock); err != nil {
			return fmt.Errorf("writing lockfile: %w", err)
		}

		log.Infof("Synced %d skills with %s", len(newLock.Entries), proj.ManifestPath)
		return nil
	},
}

func init() {
	syncCmd.Flags().BoolVar(&syncOffline, "offline", false,
		"Never access the network; use only the local cache (or set SKMGR_OFFLINE=1)")
	dryRunFlag(syncCmd, &syncDryRun, "the sync")
	rootCmd.AddCommand(syncCmd)
}
