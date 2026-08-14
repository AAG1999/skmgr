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

//go:build windows

package linker

import (
	"fmt"
	"os/exec"
	"strings"
)

// createJunction makes a directory junction, the one kind of link Windows
// allows an unprivileged user to create. Agents follow it exactly as they would
// a symlink, so it is a complete substitute for our purposes.
//
// Junctions only work for directories and only with absolute local paths, both
// of which hold for the canonical store.
func createJunction(target string, linkPath string) error {
	// mklink is a cmd.exe builtin, not an executable, so it has to be invoked
	// through the shell.
	cmd := exec.Command("cmd", "/c", "mklink", "/J", linkPath, target)

	output, err := cmd.CombinedOutput()
	if err != nil {
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return fmt.Errorf("mklink /J: %w", err)
		}
		return fmt.Errorf("mklink /J: %w: %s", err, detail)
	}

	return nil
}
