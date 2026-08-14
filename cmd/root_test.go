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
	"strings"
	"testing"

	"github.com/AAG1999/skmgr/internal/log"
)

func TestConfigureLogging_VerboseAndQuietConflict(t *testing.T) {
	resetCommandFlags(t)

	verbose = true
	quiet = true

	err := configureLogging(rootCmd)
	if err == nil {
		t.Fatal("--verbose with --quiet should be rejected")
	}
	if !strings.Contains(err.Error(), "cannot be used together") {
		t.Errorf("error = %q, want it to explain the conflict", err.Error())
	}
}

// The verbosity flags were declared but never read; they must reach the logger.
func TestConfigureLogging_FlagsSetLevel(t *testing.T) {
	tests := []struct {
		name    string
		verbose bool
		quiet   bool
		want    log.Level
	}{
		{"default", false, false, log.LevelInfo},
		{"verbose", true, false, log.LevelDebug},
		{"quiet", false, true, log.LevelError},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resetCommandFlags(t)
			verbose = tc.verbose
			quiet = tc.quiet

			if err := configureLogging(rootCmd); err != nil {
				t.Fatalf("configureLogging returned error: %v", err)
			}
			if got := log.Default().Level(); got != tc.want {
				t.Errorf("log level = %v, want %v", got, tc.want)
			}
		})
	}

	resetCommandFlags(t)
	_ = configureLogging(rootCmd)
}

func TestConfigureLogging_RejectsUnknownLevel(t *testing.T) {
	resetCommandFlags(t)

	if err := rootCmd.PersistentFlags().Set("log-level", "chatty"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = rootCmd.PersistentFlags().Set("log-level", "info")
		rootCmd.PersistentFlags().Lookup("log-level").Changed = false
	})

	if err := configureLogging(rootCmd); err == nil {
		t.Fatal("an unknown --log-level should be rejected")
	}
}

func TestRootCommand_RegistersExpectedSubcommands(t *testing.T) {
	want := []string{"init", "add", "remove", "install", "update", "sync", "list", "clean"}

	registered := make(map[string]bool)
	for _, c := range rootCmd.Commands() {
		registered[c.Name()] = true
	}

	for _, name := range want {
		if !registered[name] {
			t.Errorf("subcommand %q is not registered", name)
		}
	}
}

func TestRootCommand_HasVersion(t *testing.T) {
	if rootCmd.Version == "" {
		t.Error("root command should expose a version")
	}
}
