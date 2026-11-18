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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"text/template"

	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/AAG1999/skmgr/internal/util/atomicfile"
	"github.com/AAG1999/skmgr/internal/workflow"
	"github.com/spf13/cobra"
)

const initManifestTemplate = `name: {{ .Name }}
version: "1"

{{ if .Detected -}}
# {{ .Source }}
targets:
{{ range .Detected }}  - {{ . }}
{{ end -}}
{{- else -}}
# No agent directories detected; uncomment the ones you use.
targets:
#   - cursor
#   - gemini
#   - claude-code
#   - copilot
{{ end }}
skills:
# # A whole repo. Named after the repo -> "skills".
#   - source: https://github.com/anthropics/skills.git
#     ref: main

# # One skill from a repo. Named after the path -> "skill-creator".
#   - source: https://github.com/anthropics/skills.git
#     path: skills/skill-creator
#     ref: main

# # Every skill in a directory. Each match is named after its folder.
#   - source: https://github.com/anthropics/skills.git
#     path: "skills/*"
#     ref: main

# # A local skill, under a name you pick instead of the derived one.
#   - name: my-local-skill
#     source: file://./path/to/my/local-skill

# # scope: global installs to ~/.agents/ and links into your home directory.
# # The default, scope: project, keeps everything inside this repo.
#   - source: https://github.com/anthropics/skills.git
#     path: skills/skill-creator
#     ref: main
#     scope: global
`

var (
	initTargets []string
	initDryRun  bool
	initForce   bool

	// initTerminal and initReader are replaceable in tests. Production uses
	// the real terminal so a closed or piped stdin cannot hang on the question.
	initTerminal = stdinIsTTY
	initReader   io.Reader
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize skmgr.yml and the chat workflow files",
	Long: `Create skmgr.yml and the chat workflow files for the agents you pick.

On a terminal, init asks which agents to manage and pre-selects any whose
directory already exists. Pass --targets to skip the question, which is what
CI does. Running init again updates targets and refreshes workflow files that
still match their marker. It does not change the skills list or skmgr.lock.`,
	Args: cobra.NoArgs,
	RunE: runInit,
}

// agentMarkers maps an agent name to the directory whose presence in a project
// implies that agent is in use.
var agentMarkers = []struct {
	agent string
	dir   string
}{
	{"cursor", ".cursor"},
	{"gemini", ".gemini"},
	{"claude-code", ".claude"},
	{"copilot", ".github"},
}

// detectAgents returns the agents whose marker directory already exists, in a
// stable order so a generated manifest is reproducible.
func detectAgents(root string) []string {
	var found []string
	for _, marker := range agentMarkers {
		info, err := os.Stat(filepath.Join(root, marker.dir))
		if err == nil && info.IsDir() {
			found = append(found, marker.agent)
		}
	}
	return found
}

type initReport struct {
	Targets []string          `json:"targets"`
	Files   []workflow.Change `json:"files"`
}

func runInit(cmd *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	detected := detectAgents(cwd)
	targets := append([]string{}, initTargets...)
	if len(targets) == 0 {
		if !initTerminal() {
			return &UsageError{Msg: "skmgr init needs a terminal to choose agents, or pass --targets"}
		}
		reader := initReader
		if reader == nil {
			reader = os.Stdin
		}
		targets, err = selectAgents(reader, cmd.OutOrStdout(), detected)
		if err != nil {
			return err
		}
	}
	for _, target := range targets {
		if !types.IsValidAgent(target) {
			return &UsageError{Msg: fmt.Sprintf("invalid argument %q for --targets", target)}
		}
	}

	existing, findErr := manifest.FindManifestPath(cwd)
	creating := findErr != nil
	if findErr != nil && !errors.Is(findErr, manifest.ErrNotFound) {
		return findErr
	}

	changes, err := workflow.Plan(cwd, targets, initForce)
	if err != nil {
		return err
	}
	files, err := workflow.RenderSelected(cwd, targets)
	if err != nil {
		return err
	}

	if initDryRun {
		return reportInit(cmd, targets, changes, true)
	}

	if creating {
		if err := writeFreshManifest(cwd, targets); err != nil {
			return err
		}
	} else if err := updateManifestTargets(existing, targets); err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Join(cwd, ".agents", "skills"), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(cwd, ".agents", "rules"), 0755); err != nil {
		return err
	}
	if err := workflow.Apply(cwd, changes, files); err != nil {
		return err
	}

	return reportInit(cmd, targets, changes, false)
}

func writeFreshManifest(cwd string, targets []string) error {
	data := struct {
		Name     string
		Detected []string
		Source   string
	}{
		Name:     filepath.Base(cwd),
		Detected: targets,
		Source:   "selected during init",
	}
	tmpl, err := template.New("manifest").Parse(initManifestTemplate)
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template: %w", err)
	}
	return atomicfile.Write(filepath.Join(cwd, "skmgr.yml"), buf.Bytes(), 0644)
}

func updateManifestTargets(path string, targets []string) error {
	m, err := manifest.Parse(path)
	if err != nil {
		return err
	}
	m.Targets = targets
	return manifest.Write(path, m)
}

func reportInit(cmd *cobra.Command, targets []string, changes []workflow.Change, dry bool) error {
	if jsonOutput {
		if changes == nil {
			changes = []workflow.Change{}
		}
		return emit(cmd, initReport{Targets: targets, Files: changes})
	}
	out := cmd.OutOrStdout()
	if dry {
		_, _ = fmt.Fprintln(out, "Dry run. Nothing was written.")
	}
	for _, change := range changes {
		_, _ = fmt.Fprintf(out, "%s %s\n", change.Action, change.Path)
	}
	if dry {
		return nil
	}
	_, _ = fmt.Fprintln(out, "Getting started")
	for _, action := range workflow.Actions() {
		_, _ = fmt.Fprintf(out, "  /%s\n", action.SkillName)
	}
	_, _ = fmt.Fprintln(out, "Restart your IDE to refresh skills.")
	return nil
}

// selectAgents asks which agents to manage. detected names are pre-selected,
// and an empty line keeps that selection.
func selectAgents(r io.Reader, w io.Writer, detected []string) ([]string, error) {
	names := types.SupportedAgentNames()
	pre := map[string]bool{}
	for _, name := range detected {
		pre[name] = true
	}
	_, _ = fmt.Fprintln(w, "Which agents should skmgr manage?")
	for i, name := range names {
		mark := " "
		if pre[name] {
			mark = "x"
		}
		_, _ = fmt.Fprintf(w, "  %d. [%s] %s\n", i+1, mark, name)
	}
	_, _ = fmt.Fprint(w, "Enter numbers (comma-separated), or press Enter to accept the selection: ")

	scanner := bufio.NewScanner(r)
	if !scanner.Scan() {
		return append([]string{}, detected...), nil
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return append([]string{}, detected...), nil
	}
	var chosen []string
	seen := map[string]bool{}
	for _, part := range strings.Split(line, ",") {
		part = strings.TrimSpace(part)
		n, err := strconv.Atoi(part)
		if err != nil || n < 1 || n > len(names) {
			return nil, &UsageError{Msg: fmt.Sprintf("invalid argument %q", part)}
		}
		name := names[n-1]
		if seen[name] {
			continue
		}
		seen[name] = true
		chosen = append(chosen, name)
	}
	return chosen, nil
}

func stdinIsTTY() bool {
	info, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

func init() {
	initCmd.Flags().StringSliceVar(&initTargets, "targets", nil, "Agents to manage, skipping the prompt (comma-separated)")
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Show the workflow file changes without writing them")
	initCmd.Flags().BoolVar(&initForce, "force", false, "Overwrite workflow files that have been edited by hand")
	jsonFlag(initCmd)
	rootCmd.AddCommand(initCmd)
}
