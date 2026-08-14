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

// skmgr is the framework-agnostic skill manager for AI agents.
//
// It manages AI agent skills and rules as declarative dependencies pulled
// from any git repository. Skills are stored canonically in .agents/ and
// symlinked to each agent's native directory to avoid duplication.
package main

import "github.com/AAG1999/skmgr/cmd"

func main() {
	cmd.Execute()
}
