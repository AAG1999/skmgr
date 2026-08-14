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

//go:build !windows

package linker

import "errors"

// createJunction has no meaning outside Windows. Everywhere else a symlink
// failure is a real permission problem, so the caller should move on to
// copying rather than retry the same operation under another name.
func createJunction(target string, linkPath string) error {
	return errors.New("directory junctions are a Windows feature")
}
