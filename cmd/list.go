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

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var listJSON bool

// skillStatus is one row of `skmgr list`, and also the JSON shape.
type skillStatus struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Scope     string   `json:"scope"`
	Source    string   `json:"source"`
	Path      string   `json:"path,omitempty"`
	Ref       string   `json:"ref,omitempty"`
	CommitSHA string   `json:"commit_sha,omitempty"`
	Status    string   `json:"status"`
	Targets   []string `json:"targets"`
}

const (
	statusInstalled = "installed"
	statusMissing   = "missing"

	// statusModified means the skill is installed but its contents no longer
	// hash to what the lockfile recorded, so it is not what the team pinned.
	statusModified = "modified"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List declared skills and whether they are installed",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		// Status is checked against disk, not just against the lockfile: an
		// entry in the lock says what should be installed, not what is.
		e := engine.NewEngine(proj.Root, types.CacheDir())
		rows := collectSkillStatuses(proj.Manifest, proj.Lock, e.Verify(proj.Lock))

		if globalScope {
			rows = onlyScope(rows, types.ScopeGlobal)
		}

		if listJSON {
			// An empty result is an empty array, never null: a consumer should
			// be able to iterate the output without a nil check.
			if rows == nil {
				rows = []skillStatus{}
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(rows)
		}

		if len(rows) == 0 {
			if globalScope {
				log.Infof("No globally scoped skills declared in %s", proj.ManifestPath)
			} else {
				log.Infof("No skills declared in %s", proj.ManifestPath)
			}
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tTYPE\tSCOPE\tREF\tSTATUS\tTARGETS")
		for _, row := range rows {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				row.Name, row.Type, row.Scope, row.Ref, row.Status, strings.Join(row.Targets, ", "))
		}
		return w.Flush()
	},
}

// collectSkillStatuses pairs each declared skill with its lock entry and its
// verified state on disk.
//
// A wildcard entry has no name in the manifest; its installed members only
// appear in the lockfile, so they are listed from there.
func collectSkillStatuses(m *types.Manifest, lock *types.Lockfile, verified []engine.VerifyResult) []skillStatus {
	byName := make(map[string]engine.VerifyResult, len(verified))
	for _, r := range verified {
		byName[r.Name] = r
	}

	var rows []skillStatus

	for _, skill := range m.Skills {
		targets := skill.EffectiveTargets(m.Targets)

		if skill.Name == "" {
			rows = append(rows, wildcardStatuses(skill, lock, targets, byName)...)
			continue
		}

		row := skillStatus{
			Name:    skill.Name,
			Type:    skill.EffectiveType(),
			Scope:   skill.EffectiveScope(),
			Source:  skill.Source,
			Path:    skill.Path,
			Ref:     skill.Ref,
			Status:  statusMissing,
			Targets: targets,
		}

		if lock != nil {
			if entry := lock.FindEntry(skill.Name); entry != nil {
				row.CommitSHA = entry.CommitSHA
			}
		}
		row.Status = diskStatus(byName, skill.Name)

		rows = append(rows, row)
	}

	return rows
}

// diskStatus translates a verification result into the word `list` shows. A
// name with no result is either absent from the lockfile or not installed;
// either way the user has not got it.
func diskStatus(verified map[string]engine.VerifyResult, name string) string {
	result, ok := verified[name]
	if !ok {
		return statusMissing
	}

	switch result.Status {
	case engine.VerifyOK, engine.VerifyUnverifiable:
		return statusInstalled
	case engine.VerifyModified:
		return statusModified
	default:
		return statusMissing
	}
}

// wildcardStatuses expands a wildcard manifest entry into the lock entries it
// produced, so `list` shows what is actually installed rather than the pattern.
func wildcardStatuses(
	skill types.SkillDependency,
	lock *types.Lockfile,
	targets []string,
	verified map[string]engine.VerifyResult,
) []skillStatus {
	if lock == nil {
		return []skillStatus{{
			Name:    skill.Path,
			Type:    skill.EffectiveType(),
			Scope:   skill.EffectiveScope(),
			Source:  skill.Source,
			Path:    skill.Path,
			Ref:     skill.Ref,
			Status:  statusMissing,
			Targets: targets,
		}}
	}

	var rows []skillStatus
	for _, entry := range lock.Entries {
		if entry.Source != skill.Source || entry.Path != skill.Path {
			continue
		}
		rows = append(rows, skillStatus{
			Name:      entry.Name,
			Type:      skill.EffectiveType(),
			Scope:     skill.EffectiveScope(),
			Source:    entry.Source,
			Path:      entry.Path,
			Ref:       skill.Ref,
			CommitSHA: entry.CommitSHA,
			Status:    diskStatus(verified, entry.Name),
			Targets:   targets,
		})
	}

	if len(rows) == 0 {
		rows = append(rows, skillStatus{
			Name:    skill.Path,
			Type:    skill.EffectiveType(),
			Scope:   skill.EffectiveScope(),
			Source:  skill.Source,
			Path:    skill.Path,
			Ref:     skill.Ref,
			Status:  statusMissing,
			Targets: targets,
		})
	}

	return rows
}

// onlyScope keeps the rows belonging to one scope.
func onlyScope(rows []skillStatus, scope string) []skillStatus {
	filtered := make([]skillStatus, 0, len(rows))
	for _, row := range rows {
		if row.Scope == scope {
			filtered = append(filtered, row)
		}
	}
	return filtered
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	addGlobalFlag(listCmd, "List only globally scoped skills (~/.agents/)")
	rootCmd.AddCommand(listCmd)
}
