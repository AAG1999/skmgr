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

package manifest

import (
	"strings"
	"testing"
)

func TestParse_RejectsReservedWorkflowName(t *testing.T) {
	_, err := ParseBytes([]byte(`version: "1"
name: test
skills:
  - name: skmgr-find-skill
    source: https://example.test/skills.git
`))
	if err == nil {
		t.Fatal("reserved name was accepted")
	}
	if !strings.Contains(err.Error(), "reserved") || !strings.Contains(err.Error(), "line ") || !strings.Contains(err.Error(), "name:") {
		t.Fatalf("error should name the line, the field, and the reservation, got %v", err)
	}
}
