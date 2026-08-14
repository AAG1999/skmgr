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
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/AAG1999/skmgr/internal/config"
	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/spf13/cobra"
)

// Version is set at build time via -ldflags.
var Version = "dev"

// Global flags accessible to all subcommands.
var (
	globalScope bool
	verbose     bool
	quiet       bool
	logLevel    string
)

// addGlobalFlag registers --global on a command that acts on a single scope.
//
// It used to be a persistent flag on the root command, where it was accepted
// everywhere but read by almost nothing: `skmgr install --global` was silently
// ignored, which is worse than rejecting it. Only the commands that genuinely
// choose a scope declare it now.
//
// Scope for install, sync and update comes from each entry's `scope:` in the
// manifest rather than from a flag, so that a checkout installs the same way
// for everyone regardless of how the command was typed.
func addGlobalFlag(cmd *cobra.Command, usage string) {
	cmd.Flags().BoolVarP(&globalScope, "global", "g", false, usage)
}

// configureLogging translates the verbosity flags into a logger level.
//
// Precedence is flag, then environment or config file, then the default.
// --verbose and --quiet are mutually exclusive; an explicit --log-level wins
// over both so scripts can be precise.
func configureLogging(cmd *cobra.Command) error {
	if verbose && quiet {
		return fmt.Errorf("--verbose and --quiet cannot be used together")
	}

	if cmd.Flags().Changed("log-level") || cmd.Root().PersistentFlags().Changed("log-level") {
		level, err := log.ParseLevel(logLevel)
		if err != nil {
			return err
		}
		log.SetLevel(level)
		return nil
	}

	switch {
	case verbose:
		log.SetLevel(log.LevelDebug)
	case quiet:
		log.SetLevel(log.LevelError)
	default:
		configured := config.Current().LogLevel
		if configured == "" {
			log.SetLevel(log.LevelInfo)
			return nil
		}

		level, err := log.ParseLevel(configured)
		if err != nil {
			return fmt.Errorf("%s: %w", configuredBy(config.EnvLogLevel), err)
		}
		log.SetLevel(level)
	}

	return nil
}

// configuredBy names where a bad setting came from, since "invalid log level"
// is unhelpful when the user did not type it on this command line.
func configuredBy(env string) string {
	if os.Getenv(env) != "" {
		return env
	}
	if path := config.Current().Path; path != "" {
		return path
	}
	return env
}

// offlineRequested combines the --offline flag with the SKMGR_OFFLINE
// environment variable and the config file. Any one of them is enough; there is
// no way to ask for network access back, because a machine configured as
// offline should stay that way.
func offlineRequested(flag bool) bool {
	return flag || config.Current().Offline
}

// rootCmd is the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:   "skmgr",
	Short: "The framework-agnostic skill manager for AI agents",
	Long: `skmgr manages AI agent skills and rules as declarative dependencies
pulled from any git repository. Declare them in skmgr.yml, run skmgr install,
and every team member gets the same agent setup.

Skills are stored canonically in .agents/skills/ and symlinked to each
agent's native directory (.cursor/skills/, .claude/skills/, etc.) to
avoid duplication.`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// A config file the user wrote but skmgr could not parse has to stop
		// the run: continuing would act on settings they think are in effect.
		if err := config.Err(); err != nil {
			return err
		}
		return configureLogging(cmd)
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

// Root returns the root command.
//
// It exists for the release tooling, which generates completions and man pages
// from the live command tree so they cannot drift from the flags that actually
// exist.
func Root() *cobra.Command {
	return rootCmd
}

// Execute runs the root command. Called from main.go.
func Execute() {
	err := rootCmd.Execute()
	if err == nil {
		return
	}

	// A command that already printed a full report exits with its status and
	// nothing more; anything else gets the usual one-line error.
	var silent *silentError
	if errors.As(err, &silent) {
		os.Exit(silent.ExitStatus())
	}

	fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	os.Exit(exitStatusFor(err))
}

// exitStatusFor maps an error to a documented exit code.
func exitStatusFor(err error) int {
	var drift *engine.HashMismatchError
	if errors.As(err, &drift) {
		return ExitDrift
	}

	// Cobra reports a bad flag or a wrong argument count as a plain error, and
	// those are the user's typing rather than a failure of the operation.
	message := err.Error()
	for _, usage := range []string{
		"unknown flag",
		"unknown shorthand flag",
		"unknown command",
		"flag needs an argument",
		"invalid argument",
		"accepts ",
		"requires at least",
		"requires exactly",
	} {
		if strings.Contains(message, usage) {
			return ExitUsage
		}
	}

	return ExitError
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output (implies --log-level=debug)")
	rootCmd.PersistentFlags().BoolVarP(&quiet, "quiet", "q", false, "Suppress non-error output (implies --log-level=error)")
	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level: debug, info, warn, error")

	rootCmd.Version = Version
	rootCmd.SetVersionTemplate("skmgr version {{.Version}}\n")
}
