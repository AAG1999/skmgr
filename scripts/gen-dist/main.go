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

// Command gen-dist writes the release extras: shell completions and man pages.
//
// Both are generated from the live command tree rather than written by hand, so
// a new command or flag cannot ship with documentation that has quietly gone
// stale. GoReleaser runs this before building and attaches the output to the
// release.
//
// Usage:
//
//	go run ./scripts/gen-dist -out extras
//
// The output must stay outside dist/: GoReleaser runs its before hooks first
// and then empties dist/, so anything written there would be gone by the time
// the archives are assembled.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/AAG1999/skmgr/cmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func main() {
	out := flag.String("out", "extras", "directory to write the extras into")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintf(os.Stderr, "gen-dist: %v\n", err)
		os.Exit(1)
	}
}

func run(out string) error {
	root := cmd.Root()

	if err := writeCompletions(root, filepath.Join(out, "completions")); err != nil {
		return err
	}
	if err := writeManPages(root, filepath.Join(out, "man")); err != nil {
		return err
	}

	fmt.Printf("wrote completions and man pages to %s\n", out)
	return nil
}

// writeCompletions emits one script per supported shell.
func writeCompletions(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	generators := map[string]func(string) error{
		"skmgr.bash": func(path string) error { return root.GenBashCompletionFileV2(path, true) },
		"skmgr.zsh":  root.GenZshCompletionFile,
		"skmgr.fish": func(path string) error { return root.GenFishCompletionFile(path, true) },
		"skmgr.ps1":  root.GenPowerShellCompletionFile,
	}

	for name, generate := range generators {
		if err := generate(filepath.Join(dir, name)); err != nil {
			return fmt.Errorf("generating %s: %w", name, err)
		}
	}

	return nil
}

// writeManPages emits one page per command.
//
// The roff is written directly rather than through a markdown converter, which
// would add a dependency chain to the binary's module graph for something only
// the release job ever runs.
func writeManPages(root *cobra.Command, dir string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	date := releaseDate()

	for _, command := range allCommands(root) {
		if command.Hidden {
			continue
		}

		name := strings.ReplaceAll(command.CommandPath(), " ", "-")
		path := filepath.Join(dir, name+".1")

		if err := os.WriteFile(path, []byte(manPage(command, date)), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	return nil
}

// releaseDate is the date stamped into each page. SOURCE_DATE_EPOCH is honored
// so a reproducible build produces byte-identical pages.
func releaseDate() string {
	if epoch := os.Getenv("SOURCE_DATE_EPOCH"); epoch != "" {
		var seconds int64
		if _, err := fmt.Sscanf(epoch, "%d", &seconds); err == nil {
			return time.Unix(seconds, 0).UTC().Format("January 2006")
		}
	}
	return time.Now().UTC().Format("January 2006")
}

// allCommands flattens the command tree.
func allCommands(root *cobra.Command) []*cobra.Command {
	commands := []*cobra.Command{root}

	for _, child := range root.Commands() {
		if !child.IsAvailableCommand() {
			continue
		}
		commands = append(commands, allCommands(child)...)
	}

	return commands
}

// manPage renders one command as roff.
func manPage(command *cobra.Command, date string) string {
	var b strings.Builder

	name := command.CommandPath()
	title := strings.ToUpper(strings.ReplaceAll(name, " ", "-"))

	fmt.Fprintf(&b, ".TH \"%s\" \"1\" \"%s\" \"skmgr\" \"skmgr Manual\"\n", title, date)

	fmt.Fprintf(&b, ".SH NAME\n%s \\- %s\n", escapeRoff(name), escapeRoff(command.Short))

	fmt.Fprintf(&b, ".SH SYNOPSIS\n.B %s\n", escapeRoff(command.UseLine()))

	description := command.Long
	if description == "" {
		description = command.Short
	}
	fmt.Fprintf(&b, ".SH DESCRIPTION\n%s\n", paragraphs(description))

	if flags := renderFlags(command.NonInheritedFlags()); flags != "" {
		fmt.Fprintf(&b, ".SH OPTIONS\n%s", flags)
	}
	if flags := renderFlags(command.InheritedFlags()); flags != "" {
		fmt.Fprintf(&b, ".SH OPTIONS INHERITED FROM PARENT COMMANDS\n%s", flags)
	}

	if command.Example != "" {
		fmt.Fprintf(&b, ".SH EXAMPLES\n%s\n", paragraphs(command.Example))
	}

	if sub := subcommandList(command); sub != "" {
		fmt.Fprintf(&b, ".SH COMMANDS\n%s", sub)
	}

	fmt.Fprintf(&b, ".SH SEE ALSO\n%s\n",
		escapeRoff("Full documentation at https://github.com/AAG1999/skmgr"))

	return b.String()
}

// renderFlags lists a flag set as a roff definition list.
func renderFlags(flags *pflag.FlagSet) string {
	var b strings.Builder

	flags.VisitAll(func(flag *pflag.Flag) {
		if flag.Hidden {
			return
		}

		label := "\\-\\-" + flag.Name
		if flag.Shorthand != "" {
			label = "\\-" + flag.Shorthand + ", " + label
		}
		if flag.Value.Type() != "bool" {
			label += " \\fI" + flag.Value.Type() + "\\fR"
		}

		fmt.Fprintf(&b, ".TP\n\\fB%s\\fR\n%s\n", label, escapeRoff(flag.Usage))
	})

	return b.String()
}

// subcommandList names the children of a command.
func subcommandList(command *cobra.Command) string {
	var b strings.Builder

	for _, child := range command.Commands() {
		if !child.IsAvailableCommand() {
			continue
		}
		fmt.Fprintf(&b, ".TP\n\\fB%s\\fR\n%s\n", escapeRoff(child.Name()), escapeRoff(child.Short))
	}

	return b.String()
}

// paragraphs turns blank-line-separated prose into roff paragraphs.
func paragraphs(text string) string {
	blocks := strings.Split(strings.TrimSpace(text), "\n\n")

	rendered := make([]string, 0, len(blocks))
	for _, block := range blocks {
		rendered = append(rendered, escapeRoff(strings.TrimSpace(block)))
	}

	return strings.Join(rendered, "\n.PP\n")
}

// escapeRoff protects the characters roff would otherwise read as markup. A
// line starting with a period is a request, so text beginning with one has to
// be guarded or the line disappears from the rendered page.
func escapeRoff(text string) string {
	text = strings.ReplaceAll(text, `\`, `\e`)

	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, ".") || strings.HasPrefix(line, "'") {
			lines[i] = `\&` + line
		}
	}

	return strings.Join(lines, "\n")
}
