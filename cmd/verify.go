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
	"text/tabwriter"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var verifyJSON bool

// verifyRow is the JSON shape of one verification result.
type verifyRow struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Location string `json:"location,omitempty"`
}

var verifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Check that installed skills match the lockfile",
	Long: `Compare every entry in skmgr.lock against what is installed on disk.

verify hashes the installed directories and compares them to the hashes the
lockfile recorded. It never contacts the network, so it is safe to run in CI or
offline.

Exits non-zero if anything is missing or modified.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		if proj.Lock == nil {
			return fmt.Errorf("no %s to verify against (run 'skmgr install' first)", LockfileName)
		}

		e := engine.NewEngine(proj.Root, types.CacheDir())
		results := e.Verify(proj.Lock)

		if verifyJSON {
			rows := make([]verifyRow, 0, len(results))
			for _, r := range results {
				rows = append(rows, verifyRow{
					Name:     r.Name,
					Status:   string(r.Status),
					Expected: r.Expected,
					Actual:   r.Actual,
					Location: r.Location,
				})
			}
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(rows); err != nil {
				return err
			}
			return verifyExitError(results)
		}

		if len(results) == 0 {
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Nothing to verify: the lockfile has no entries")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		_, _ = fmt.Fprintln(w, "NAME\tSTATUS\tDETAIL")
		for _, r := range results {
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", r.Name, r.Status, verifyDetail(r))
		}
		if err := w.Flush(); err != nil {
			return err
		}

		return verifyExitError(results)
	},
}

// verifyDetail explains a result in the terms the user needs to act on.
func verifyDetail(r engine.VerifyResult) string {
	switch r.Status {
	case engine.VerifyOK:
		return ""
	case engine.VerifyMissing:
		return "not installed; run 'skmgr install'"
	case engine.VerifyModified:
		return fmt.Sprintf("content changed since it was locked (%s)", r.Location)
	case engine.VerifyUnverifiable:
		return "lockfile records no hash for this entry"
	default:
		return string(r.Status)
	}
}

// verifyExitError turns failures into a non-zero exit without printing a
// second, redundant description of rows the table already listed.
//
// The status is ExitDrift rather than a plain failure, so a pipeline can fail
// a pull request because a skill was edited without also failing when skmgr
// itself could not run.
func verifyExitError(results []engine.VerifyResult) error {
	var missing, modified int
	for _, r := range results {
		switch r.Status {
		case engine.VerifyMissing:
			missing++
		case engine.VerifyModified:
			modified++
		}
	}

	if missing == 0 && modified == 0 {
		return nil
	}

	return &silentError{
		code:   fmt.Sprintf("verification failed: %d missing, %d modified", missing, modified),
		status: ExitDrift,
	}
}

func init() {
	verifyCmd.Flags().BoolVar(&verifyJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(verifyCmd)
}
