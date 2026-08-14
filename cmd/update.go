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
	"strings"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	updateOffline bool
	updateLatest  bool
	updateDryRun  bool
)

var updateCmd = &cobra.Command{
	Use:   "update [name...]",
	Short: "Update one or all skills to the latest matching version",
	Long: `Re-resolve skills against their declared refs and reinstall them.

Given one or more names, only those skills move; every other skill is held at
the commit already recorded in skmgr.lock. With no names, all skills update.

A skill whose ref is a version range stays inside that range. Use --latest to
take the newest release even when it falls outside, which is how you cross a
major version deliberately. The manifest is not rewritten, so the range still
applies on the next plain update.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		if len(args) > 0 && proj.Lock == nil {
			return fmt.Errorf("cannot update named skills before an initial install: no %s found", LockfileName)
		}

		opts := engine.SyncOptions{
			Only:    args,
			Offline: offlineRequested(updateOffline),
			Latest:  updateLatest,
		}

		if updateDryRun {
			return runDryRun(cmd, proj, proj.Manifest, opts)
		}

		e := engine.NewEngine(proj.Root, types.CacheDir())

		if len(args) > 0 {
			log.Debugf("updating only: %s", strings.Join(args, ", "))
		}

		newLock, err := e.Sync(proj.Manifest, proj.Lock, opts)
		if err != nil {
			return fmt.Errorf("update failed: %w", err)
		}

		if err := lockfile.Write(proj.LockPath, newLock); err != nil {
			return fmt.Errorf("writing lockfile: %w", err)
		}

		reportUpdated(proj.Lock, newLock, args)
		return nil
	},
}

// reportUpdated tells the user which skills actually moved, so a no-op update
// is distinguishable from one that changed something.
func reportUpdated(oldLock *types.Lockfile, newLock *types.Lockfile, requested []string) {
	var changed []string

	for _, entry := range newLock.Entries {
		var previous *types.LockEntry
		if oldLock != nil {
			previous = oldLock.FindEntry(entry.Name)
		}

		switch {
		case previous == nil:
			changed = append(changed, fmt.Sprintf("%s (new)", entry.Name))
		case previous.CommitSHA != entry.CommitSHA || previous.ContentHash != entry.ContentHash:
			changed = append(changed, describeChange(*previous, entry))
		}
	}

	scope := "all skills"
	if len(requested) > 0 {
		scope = strings.Join(requested, ", ")
	}

	if len(changed) == 0 {
		log.Infof("Update complete (%s): already up to date", scope)
		return
	}

	log.Infof("Update complete (%s): changed %s", scope, strings.Join(changed, ", "))
}

// describeChange names the version movement when there is one to name. A tag
// bump is far more useful to read than a bare skill name, and a range makes it
// unclear which version you are on without it.
func describeChange(previous types.LockEntry, current types.LockEntry) string {
	from, to := previous.EffectiveRef(), current.EffectiveRef()

	if from != "" && to != "" && from != to {
		return fmt.Sprintf("%s (%s -> %s)", current.Name, from, to)
	}

	return current.Name
}

func init() {
	updateCmd.Flags().BoolVar(&updateOffline, "offline", false,
		"Never access the network; use only the local cache (or set SKMGR_OFFLINE=1)")
	updateCmd.Flags().BoolVar(&updateLatest, "latest", false,
		"Take the newest release tag even if it falls outside the declared range")
	dryRunFlag(updateCmd, &updateDryRun, "the update")
	rootCmd.AddCommand(updateCmd)
}
