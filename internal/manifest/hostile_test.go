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
	"time"
)

// parseBudget is how long any single manifest may take.
//
// A manifest is often fetched from a repository someone else controls, and
// skmgr runs in CI, so a file that takes minutes to parse is a way to stall a
// pipeline. Real manifests parse in microseconds; a second is generous enough
// that a loaded machine will not make this flaky.
//
// The budget is a ceiling, not a wait: a passing parse returns immediately, so
// scaling it for the race detector costs nothing and still catches the
// unbounded blowups these tests are looking for.
const parseBudget = time.Second * raceSlowdown

// parseWithin runs the parser and fails if it does not finish in time.
//
// The parse runs in its own goroutine because a genuine hang would otherwise
// take the whole test binary down with a timeout panic, which reports the
// symptom without naming the input.
func parseWithin(t *testing.T, label string, data string) {
	t.Helper()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = ParseBytes([]byte(data))
	}()

	select {
	case <-done:
	case <-time.After(parseBudget):
		t.Fatalf("%s did not parse within %s; a hostile manifest can stall a build", label, parseBudget)
	}
}

// A YAML alias can reference another alias, so a small file can describe an
// enormous structure. This is the "billion laughs" attack, and a parser without
// a limit will happily try to build all of it.
func TestParseBytes_ResistsAnAliasBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: \"1\"\nname: demo\n")
	b.WriteString("a: &a [\"x\",\"x\",\"x\",\"x\",\"x\",\"x\",\"x\",\"x\",\"x\"]\n")

	previous := "a"
	for i := 0; i < 9; i++ {
		key := string(rune('b' + i))
		b.WriteString(key + ": &" + key + " [")
		for j := 0; j < 9; j++ {
			if j > 0 {
				b.WriteString(",")
			}
			b.WriteString("*" + previous)
		}
		b.WriteString("]\n")
		previous = key
	}

	parseWithin(t, "an alias bomb", b.String())
}

func TestParseBytes_ResistsASelfReferencingAlias(t *testing.T) {
	parseWithin(t, "a self-referencing alias", "version: \"1\"\nname: demo\na: &x [*x]\n")
}

func TestParseBytes_ResistsDeepNesting(t *testing.T) {
	parseWithin(t, "deeply nested flow sequences",
		"version: \"1\"\nname: demo\nskills: "+strings.Repeat("[", 100_000))
}

func TestParseBytes_ResistsAHugeScalar(t *testing.T) {
	parseWithin(t, "a very long name", "version: \"1\"\nname: "+strings.Repeat("a", 1_000_000)+"\n")
}

func TestParseBytes_ResistsManySkills(t *testing.T) {
	var b strings.Builder
	b.WriteString("version: \"1\"\nname: demo\nskills:\n")
	for i := 0; i < 20_000; i++ {
		b.WriteString("  - source: ./s\n")
		b.WriteString("    name: skill-")
		b.WriteString(string(rune('a' + i%26)))
		b.WriteString(itoa(i))
		b.WriteString("\n")
	}

	parseWithin(t, "twenty thousand skills", b.String())
}

// itoa avoids pulling strconv into a file that otherwise needs nothing.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}

	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
