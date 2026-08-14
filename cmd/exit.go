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

// Exit codes. A script should be able to tell "your project is out of date"
// apart from "skmgr could not run", so the states a CI job branches on get
// their own codes instead of all collapsing into 1.
const (
	// ExitOK means the command did what was asked.
	ExitOK = 0

	// ExitError is any ordinary failure: a bad manifest, an unreachable
	// remote, a permission problem.
	ExitError = 1

	// ExitUsage means the command line itself was wrong.
	ExitUsage = 2

	// ExitDrift means the command ran fine and found the project out of line
	// with what was declared: a failed verify, a doctor check, or a hash that
	// does not match the lockfile under --frozen.
	ExitDrift = 3
)

// silentError carries an exit status for a command that has already explained
// itself in full.
//
// Without it, `doctor` would print its report and then a redundant
// "Error: 2 check(s) failed" underneath, which reads as though something else
// went wrong.
type silentError struct {
	// code is kept for the error interface and for tests that assert on the
	// reason; it is never printed by Execute.
	code string

	// status is the process exit status, defaulting to ExitDrift.
	status int
}

func (e *silentError) Error() string {
	return e.code
}

// ExitStatus reports the status this error should terminate with.
func (e *silentError) ExitStatus() int {
	if e.status == 0 {
		return ExitDrift
	}
	return e.status
}
