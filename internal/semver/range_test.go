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

package semver

import "testing"

func TestIsRange(t *testing.T) {
	ranges := []string{
		"latest",
		"LATEST",
		"^1.2.0",
		"~1.2",
		">=1.0.0",
		"<2.0.0",
		">1.0.0",
		"<=1.5.0",
		"=1.2.3",
		"*",
		"x",
		"1.x",
		"1.X",
		"1.2.*",
		"v1.x",
		"1.2 - 1.4",
		"1.x || 2.x",
		">=1.0.0 <2.0.0",
	}

	for _, ref := range ranges {
		if !IsRange(ref) {
			t.Errorf("IsRange(%q) = false, want true", ref)
		}
	}

	// A literal ref must never be mistaken for a range: doing so would install
	// a different commit than the one the user named.
	literals := []string{
		"",
		"main",
		"master",
		"develop",
		"v1.2.3",
		"1.2.3",
		"1.2.3-beta.1",
		"release-2",
		"feature/x",
		"feature/1.x-support",
		"my branch",
		"4f2c1ab9e0d3f1a2b3c4d5e6f708192a3b4c5d6e",
		"xylophone",
	}

	for _, ref := range literals {
		if IsRange(ref) {
			t.Errorf("IsRange(%q) = true, want false", ref)
		}
	}
}

func TestHasVersionWildcard(t *testing.T) {
	cases := map[string]bool{
		"*":        true,
		"x":        true,
		"X":        true,
		"1.x":      true,
		"1.2.x":    true,
		"v1.x":     true,
		"1.*":      true,
		"1.2.3":    false,
		"1":        false,
		"main":     false,
		"x.y.z":    false,
		"1.x.beta": false,
	}

	for token, want := range cases {
		if got := hasVersionWildcard(token); got != want {
			t.Errorf("hasVersionWildcard(%q) = %v, want %v", token, got, want)
		}
	}
}
