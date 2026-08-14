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
	"path/filepath"
	"testing"

	"github.com/AAG1999/skmgr/internal/config"
)

func TestOfflineRequested(t *testing.T) {
	cases := []struct {
		name string
		flag bool
		env  string
		want bool
	}{
		{name: "neither", flag: false, env: "", want: false},
		{name: "flag only", flag: true, env: "", want: true},
		{name: "env 1", flag: false, env: "1", want: true},
		{name: "env true", flag: false, env: "true", want: true},
		{name: "env yes", flag: false, env: "yes", want: true},
		{name: "env 0 is off", flag: false, env: "0", want: false},
		{name: "env false is off", flag: false, env: "false", want: false},
		{name: "env no is off", flag: false, env: "no", want: false},
		{name: "whitespace is off", flag: false, env: "  ", want: false},
		// A machine configured as offline stays offline; there is no opting
		// back into the network from the command line.
		{name: "env wins over an unset flag", flag: false, env: "1", want: true},
		{name: "flag wins over a disabled env", flag: true, env: "0", want: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(config.EnvConfig, filepath.Join(t.TempDir(), "absent.yml"))
			t.Setenv(config.EnvOffline, tc.env)
			config.Reset()
			t.Cleanup(config.Reset)

			if got := offlineRequested(tc.flag); got != tc.want {
				t.Errorf("offlineRequested(%v) with %s=%q = %v, want %v",
					tc.flag, config.EnvOffline, tc.env, got, tc.want)
			}
		})
	}
}
