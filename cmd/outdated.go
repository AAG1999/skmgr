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
	"strings"
	"text/tabwriter"

	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/provider"
	"github.com/AAG1999/skmgr/internal/semver"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	outdatedJSON    bool
	outdatedOffline bool
)

// outdatedRow reports one skill's position relative to what its repository
// publishes.
//
// Two "latest" columns are needed because they answer different questions:
// WantedVersion is what a plain `skmgr update` would install, and
// LatestVersion is what exists at all. A gap between them is exactly the
// major-version bump that a range is there to protect you from.
type outdatedRow struct {
	Name           string `json:"name"`
	Source         string `json:"source"`
	Range          string `json:"range,omitempty"`
	CurrentVersion string `json:"current,omitempty"`
	WantedVersion  string `json:"wanted,omitempty"`
	LatestVersion  string `json:"latest,omitempty"`
	Note           string `json:"note,omitempty"`
}

// Outdated reports whether this row is behind what its range allows.
func (r outdatedRow) Outdated() bool {
	return r.WantedVersion != "" && r.CurrentVersion != r.WantedVersion
}

var outdatedCmd = &cobra.Command{
	Use:   "outdated",
	Short: "Show which skills have newer versions available",
	Long: `Compare each installed skill against the tags its repository publishes.

Three versions are reported: the one installed, the newest that satisfies the
declared range, and the newest that exists. A skill is listed as outdated only
when a newer version fits its range, since that is what 'skmgr update' would
install. A newer version outside the range is shown for information; taking it
needs 'skmgr update --latest' or an edited manifest.

Only skills whose ref is a version range can be compared this way. A skill
pinned to a branch or a commit has nothing to compare against and is reported
as such.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		if len(proj.Manifest.Skills) == 0 {
			log.Infof("No skills declared in %s", proj.ManifestPath)
			return nil
		}

		rows := collectOutdated(proj, provider.FetchOptions{
			Offline: offlineRequested(outdatedOffline),
		})

		if outdatedJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(rows)
		}

		printOutdated(cmd, rows)
		return nil
	},
}

// collectOutdated builds one row per declared skill.
func collectOutdated(proj *projectContext, opts provider.FetchOptions) []outdatedRow {
	// One repository can back several skills, so its tags are fetched once.
	tagsBySource := make(map[string][]string)
	errBySource := make(map[string]error)

	var rows []outdatedRow

	for _, skill := range proj.Manifest.Skills {
		row := outdatedRow{
			Name:   skill.Name,
			Source: skill.Source,
			Range:  skill.Ref,
		}

		if entry := findLockEntry(proj.Lock, skill); entry != nil {
			row.CurrentVersion = entry.EffectiveRef()
		}

		if !semver.IsRange(skill.Ref) {
			row.Note = "not version-ranged"
			row.Range = ""
			rows = append(rows, row)
			continue
		}

		if _, done := tagsBySource[skill.Source]; !done {
			tags, err := provider.ListTags(skill.Source, types.CacheDir(), opts)
			tagsBySource[skill.Source] = tags
			errBySource[skill.Source] = err
		}

		if err := errBySource[skill.Source]; err != nil {
			row.Note = fmt.Sprintf("could not read tags: %v", err)
			rows = append(rows, row)
			continue
		}

		tags := tagsBySource[skill.Source]

		if wanted, err := semver.Resolve(tags, skill.Ref); err == nil {
			row.WantedVersion = wanted
		} else {
			row.Note = err.Error()
		}

		if latest, err := semver.Newest(tags); err == nil {
			row.LatestVersion = latest
		}

		rows = append(rows, row)
	}

	return rows
}

// findLockEntry locates a manifest entry's lock record. Names are matched
// first; source and path are the fallback for a skill whose name was derived
// during resolution, such as a wildcard expansion.
func findLockEntry(lock *types.Lockfile, skill types.SkillDependency) *types.LockEntry {
	if lock == nil {
		return nil
	}

	if skill.Name != "" {
		if entry := lock.FindEntry(skill.Name); entry != nil {
			return entry
		}
	}

	for i := range lock.Entries {
		if lock.Entries[i].Source == skill.Source && lock.Entries[i].Path == skill.Path {
			return &lock.Entries[i]
		}
	}

	return nil
}

func printOutdated(cmd *cobra.Command, rows []outdatedRow) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "NAME\tRANGE\tCURRENT\tWANTED\tLATEST\tNOTE")

	for _, row := range rows {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			row.Name,
			orDash(row.Range),
			orDash(row.CurrentVersion),
			orDash(row.WantedVersion),
			orDash(row.LatestVersion),
			row.Note)
	}
	_ = w.Flush()

	var behind, crossable []string
	for _, row := range rows {
		if row.Outdated() {
			behind = append(behind, row.Name)
		}
		// A newer version outside the range is worth pointing out, but only as
		// information: it is not what the manifest agreed to install.
		if row.LatestVersion != "" && row.WantedVersion != "" && row.LatestVersion != row.WantedVersion {
			crossable = append(crossable, fmt.Sprintf("%s %s", row.Name, row.LatestVersion))
		}
	}

	if len(behind) == 0 {
		log.Infof("Up to date within the declared ranges")
	} else {
		log.Infof("Run 'skmgr update' to move %s", strings.Join(behind, ", "))
	}

	if len(crossable) > 0 {
		log.Infof("Outside the declared range: %s. Use 'skmgr update --latest' to take it",
			strings.Join(crossable, ", "))
	}
}

// orDash renders an empty column as a dash so rows stay readable.
func orDash(value string) string {
	if value == "" {
		return "-"
	}
	return value
}

func init() {
	outdatedCmd.Flags().BoolVar(&outdatedJSON, "json", false, "Output as JSON")
	outdatedCmd.Flags().BoolVar(&outdatedOffline, "offline", false,
		"Never access the network; use only the local cache (or set SKMGR_OFFLINE=1)")
	rootCmd.AddCommand(outdatedCmd)
}
