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

package provider

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/AAG1999/skmgr/internal/log"
)

// gitRetryAttempts and gitRetryBackoff control how hard we try before giving
// up on a network git operation. A CI runner losing a packet should not fail
// the build, but a genuinely wrong URL should fail quickly rather than making
// the user wait through a long backoff.
var (
	gitRetryAttempts = 3
	gitRetryBackoff  = time.Second
)

// runGitWithRetry runs a git command, retrying transient network failures with
// exponential backoff.
//
// The command is rebuilt for each attempt because an exec.Cmd cannot be reused
// once it has run.
func runGitWithRetry(label string, build func() *exec.Cmd) error {
	backoff := gitRetryBackoff
	var lastErr error

	for attempt := 1; attempt <= gitRetryAttempts; attempt++ {
		cmd := build()
		out, err := cmd.CombinedOutput()
		if err == nil {
			return nil
		}

		lastErr = fmt.Errorf("git %s failed: %v\n%s", label, err, strings.TrimSpace(string(out)))

		if !isTransientGitFailure(string(out)) {
			return lastErr
		}

		if attempt == gitRetryAttempts {
			break
		}

		log.Warnf("git %s failed (attempt %d/%d), retrying in %s", label, attempt, gitRetryAttempts, backoff)
		time.Sleep(backoff)
		backoff *= 2
	}

	return fmt.Errorf("%w (after %d attempts)", lastErr, gitRetryAttempts)
}

// transientGitFailures are messages that indicate the network or the far end
// misbehaved, rather than the request being wrong. Retrying a bad URL or a
// missing ref only wastes the user's time.
var transientGitFailures = []string{
	"could not resolve host",
	"connection timed out",
	"connection reset",
	"connection refused",
	"operation timed out",
	"temporary failure in name resolution",
	"the remote end hung up unexpectedly",
	"rpc failed",
	"early eof",
	"unexpected disconnect",
	"ssl_error",
	"tls handshake",
	"gnutls_handshake",
	"failed to connect",
	"network is unreachable",
	"502 bad gateway",
	"503 service unavailable",
	"504 gateway timeout",
	"429 too many requests",
}

func isTransientGitFailure(output string) bool {
	lower := strings.ToLower(output)
	for _, needle := range transientGitFailures {
		if strings.Contains(lower, needle) {
			return true
		}
	}
	return false
}
