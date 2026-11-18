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
	"errors"
	"io"
	"strings"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/provider"
	"github.com/spf13/cobra"
)

// jsonOutput is set by --json on the commands that grew a machine-readable
// mode. list, tree, verify, outdated, and doctor keep their own flags so
// their existing documents do not change shape.
var jsonOutput bool

func jsonFlag(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print one JSON document on stdout")
}

// UsageError is a wrong command line. It exits 2 and uses the JSON code "usage".
type UsageError struct {
	Msg string
}

func (e *UsageError) Error() string { return e.Msg }

type errorDocument struct {
	Error errorBody `json:"error"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Fix     string `json:"fix,omitempty"`
}

type changeDocument struct {
	DryRun  bool               `json:"dry_run"`
	Changes []engine.PlanEntry `json:"changes"`
}

// emit writes one indented JSON document. Skills parse stdout and nothing else.
func emit(cmd *cobra.Command, value any) error {
	enc := json.NewEncoder(cmd.OutOrStdout())
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

// presentPlan prints a dry run as a table, or as the change document when
// --json is set. A real run calls it only in JSON mode, after the sync.
func presentPlan(cmd *cobra.Command, dry bool, plan *engine.Plan) error {
	if jsonOutput {
		changes := plan.Changes()
		if changes == nil {
			changes = []engine.PlanEntry{}
		}
		return emit(cmd, changeDocument{DryRun: dry, Changes: changes})
	}
	if dry {
		printPlan(cmd.OutOrStdout(), plan)
	}
	return nil
}

// writeError prints the JSON error document. A silent error already printed
// its report, so nothing more is written. The returned status is the process
// exit code.
func writeError(w io.Writer, err error) int {
	var silent *silentError
	if errors.As(err, &silent) {
		return silent.ExitStatus()
	}

	doc, status := classifyError(err)
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(doc)
	return status
}

func classifyError(err error) (errorDocument, int) {
	message := err.Error()

	var mismatch *engine.HashMismatchError
	if errors.As(err, &mismatch) {
		return errorDocument{errorBody{Code: "hash_mismatch", Message: message, Fix: "run skmgr install without --frozen after confirming the source change"}}, ExitDrift
	}

	var net *provider.NetworkError
	if errors.As(err, &net) {
		return errorDocument{errorBody{Code: "network", Message: message, Fix: "retry, or pass --offline if the skill is already cached"}}, ExitError
	}

	var usage *UsageError
	if errors.As(err, &usage) || isUsageMessage(message) {
		return errorDocument{errorBody{Code: "usage", Message: message}}, ExitUsage
	}

	if errors.Is(err, manifest.ErrNotFound) {
		return errorDocument{errorBody{Code: "no_manifest", Message: message, Fix: "run skmgr init"}}, ExitError
	}

	if errors.Is(err, lockfile.ErrNotFound) {
		return errorDocument{errorBody{Code: "no_lockfile", Message: message, Fix: "run skmgr install"}}, ExitError
	}

	if strings.Contains(message, "no skills found") {
		return errorDocument{errorBody{Code: "no_skills_found", Message: message, Fix: "set path: to the directory that holds the skill"}}, ExitError
	}

	if strings.Contains(message, "failed to read") || strings.Contains(message, "parsing manifest") {
		return errorDocument{errorBody{Code: "invalid_manifest", Message: message, Fix: "fix skmgr.yml and run the command again"}}, ExitError
	}

	return errorDocument{errorBody{Code: "error", Message: message}}, exitStatusFor(err)
}

func isUsageMessage(message string) bool {
	for _, usage := range []string{
		"unknown flag",
		"unknown shorthand flag",
		"unknown command",
		"flag needs an argument",
		"invalid argument",
		"accepts ",
		"requires at least",
		"requires exactly",
	} {
		if strings.Contains(message, usage) {
			return true
		}
	}
	return false
}
