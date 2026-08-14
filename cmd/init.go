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
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/AAG1999/skmgr/internal/config"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/util/atomicfile"
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

// templateSource explains in the generated manifest where the target list came
// from, so a reader is not left guessing why those agents are listed.
func templateSource(fromConfig bool) string {
	if fromConfig {
		return "Defaults from your skmgr config file."
	}
	return "Detected from directories already present in this project."
}

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a new skmgr.yml in the current directory",
	RunE: func(cmd *cobra.Command, args []string) error {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("getting current directory: %w", err)
		}

		// Refuse if either spelling is present. Discovery prefers skmgr.yml, so
		// writing one next to an existing skmgr.yaml would silently shadow it
		// and leave two manifests drifting apart.
		if existing, err := manifest.FindManifestPath(cwd); err == nil {
			return fmt.Errorf("%s already exists", filepath.Base(existing))
		}

		manifestPath := filepath.Join(cwd, "skmgr.yml")

		// Create canonical dirs
		_ = os.MkdirAll(filepath.Join(cwd, ".agents", "skills"), 0755)
		_ = os.MkdirAll(filepath.Join(cwd, ".agents", "rules"), 0755)

		type templateData struct {
			Name     string
			Detected []string
			Source   string
		}

		// Detection reads the project; the config file is the fallback for
		// someone who uses the same agents everywhere and would otherwise
		// uncomment the same lines in every new repository.
		detected := detectAgents(cwd)
		fromConfig := false
		if len(detected) == 0 {
			if configured := config.Current().Targets; len(configured) > 0 {
				detected, fromConfig = configured, true
			}
		}

		data := templateData{
			Name:     filepath.Base(cwd),
			Detected: detected,
			Source:   templateSource(fromConfig),
		}

		tmpl, err := template.New("manifest").Parse(initManifestTemplate)
		if err != nil {
			return fmt.Errorf("parsing template: %w", err)
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return fmt.Errorf("executing template: %w", err)
		}

		if err := atomicfile.Write(manifestPath, buf.Bytes(), 0644); err != nil {
			return fmt.Errorf("writing manifest: %w", err)
		}

		log.Infof("Initialized skmgr.yml for project %s", data.Name)
		if len(detected) > 0 {
			log.Infof("Detected agents: %s", strings.Join(detected, ", "))
		} else {
			log.Infof("No agent directories found; edit targets in skmgr.yml to pick your agents")
		}
		return nil
	},
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

func init() {
	rootCmd.AddCommand(initCmd)
}
