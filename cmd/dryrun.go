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
	"io"
	"text/tabwriter"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

// dryRunFlag registers --dry-run on a command that changes the project.
func dryRunFlag(cmd *cobra.Command, target *bool, what string) {
	cmd.Flags().BoolVar(target, "dry-run", false,
		fmt.Sprintf("Show what %s would do, without changing anything", what))
}

// runDryRun prints the plan for a sync-like command and reports nothing else.
//
// Both the manifest and the lockfile are left alone, so this is safe to run
// against a dirty checkout or in a pull-request check.
func runDryRun(
	cmd *cobra.Command,
	proj *projectContext,
	manifest *types.Manifest,
	opts engine.SyncOptions,
) error {
	e := engine.NewEngine(proj.Root, types.CacheDir())

	plan, err := e.Plan(manifest, proj.Lock, opts)
	if err != nil {
		return err
	}

	printPlan(cmd.OutOrStdout(), plan)
	return nil
}

// printPlan renders a plan as a table of the changes it would make.
func printPlan(out io.Writer, plan *engine.Plan) {
	changes := plan.Changes()

	if len(changes) == 0 {
		_, _ = fmt.Fprintln(out, "Nothing to do; the project already matches the manifest.")
		return
	}

	w := tabwriter.NewWriter(out, 0, 0, 3, ' ', 0)
	_, _ = fmt.Fprintln(w, "ACTION\tNAME\tCHANGE")
	for _, entry := range changes {
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", entry.Action, entry.Name, describePlanChange(entry))
	}
	_ = w.Flush()

	_, _ = fmt.Fprintf(out, "\n%s. Run without --dry-run to apply.\n", pluralizeChanges(len(changes)))
}

// describePlanChange summarizes the version movement in one column.
//
// A local source has neither a ref nor a commit, so the version is left out
// entirely rather than printed as a placeholder that says nothing.
func describePlanChange(entry engine.PlanEntry) string {
	from := shortVersion(entry.FromRef, entry.FromSHA)
	to := shortVersion(entry.ToRef, entry.ToSHA)

	switch entry.Action {
	case engine.PlanRemove:
		if from == "" {
			return "no longer declared"
		}
		return fmt.Sprintf("no longer declared (was %s)", from)

	case engine.PlanInstall:
		if to == "" {
			return "not installed yet"
		}
		return fmt.Sprintf("new at %s", to)

	case engine.PlanReinstall:
		return "contents differ from the lockfile"

	default:
		if from == "" || to == "" {
			return "source has changed"
		}
		return fmt.Sprintf("%s -> %s", from, to)
	}
}

// shortVersion prefers a tag or branch name, falling back to an abbreviated
// commit, which is all a reader needs to see that something moved.
func shortVersion(ref string, sha string) string {
	switch {
	case ref != "" && sha != "":
		return fmt.Sprintf("%s (%s)", ref, shortSHA(sha))
	case ref != "":
		return ref
	default:
		return shortSHA(sha)
	}
}

// shortSHA abbreviates a commit to the usual seven characters.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// pluralizeChanges phrases the summary line.
func pluralizeChanges(count int) string {
	if count == 1 {
		return "1 change"
	}
	return fmt.Sprintf("%d changes", count)
}
