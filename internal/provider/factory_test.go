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
	"reflect"
	"testing"
)

func TestGetProvider(t *testing.T) {
	tests := []struct {
		source string
		want   Provider
	}{
		{"https://github.com/a/b.git", &GitProvider{}},
		{"git@github.com:a/b.git", &GitProvider{}},
		{"file:///Users/me/skills", &LocalProvider{}},
		{"/Users/me/skills", &LocalProvider{}},
		{"./local-skills", &LocalProvider{}},
		{"../shared-skills", &LocalProvider{}},
		{"C:\\Users\\me\\skills", &LocalProvider{}},
		{"D:/skills", &LocalProvider{}},

		// git:: forces git treatment of a source that would otherwise be read
		// as a plain directory, which is how a local clone or mirror gets refs
		// and version ranges.
		{"git:://Users/me/mirror.git", &GitProvider{}},
		{"git::/Users/me/mirror", &GitProvider{}},
		{"git::./relative-mirror", &GitProvider{}},
		{"git::file:///Users/me/mirror", &GitProvider{}},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			got := GetProvider(tt.source)
			if reflect.TypeOf(got) != reflect.TypeOf(tt.want) {
				t.Errorf("GetProvider(%q) = %T, want %T", tt.source, got, tt.want)
			}
		})
	}
}

// git:: is skmgr's notation and means nothing to git, so it has to be removed
// before the URL is handed over.
func TestCloneURL(t *testing.T) {
	cases := map[string]string{
		"git::/Users/me/mirror":           "/Users/me/mirror",
		"git::file:///Users/me/mirror":    "file:///Users/me/mirror",
		"https://github.com/a/b.git":      "https://github.com/a/b.git",
		"/Users/me/skills":                "/Users/me/skills",
		"git::https://github.com/a/b.git": "https://github.com/a/b.git",
	}

	for source, want := range cases {
		if got := CloneURL(source); got != want {
			t.Errorf("CloneURL(%q) = %q, want %q", source, got, want)
		}
	}
}
