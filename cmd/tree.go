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
	"encoding/json"
	"fmt"
	"io"
	"sort"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var treeJSON bool

// treeSkill is one skill in the layout, and also the JSON shape.
type treeSkill struct {
	Name string `json:"name"`

	// Type is skill or rule.
	Type string `json:"type"`

	// Status is the same vocabulary `list` uses: installed, missing, modified.
	Status string `json:"status"`

	// Ref is the version in effect, preferring a resolved tag over a range.
	Ref string `json:"ref,omitempty"`

	// Source and Path say where the skill came from.
	Source string `json:"source"`
	Path   string `json:"path,omitempty"`

	// Targets are the agents this skill is linked into.
	Targets []string `json:"targets"`
}

// treeScope groups the skills installed at one scope.
type treeScope struct {
	Scope  string      `json:"scope"`
	Skills []treeSkill `json:"skills"`
}

var treeCmd = &cobra.Command{
	Use:   "tree",
	Short: "Show how skills are laid out across scopes and agents",
	Long: `Print the resolved layout: which skills exist, at which scope, and
which agents each one is linked into.

Where list is a flat table, tree shows the shape of the install, which is the
quicker way to see that a skill is missing from an agent you expected.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		proj, err := loadProject()
		if err != nil {
			return err
		}

		e := engine.NewEngine(proj.Root, types.CacheDir())
		rows := collectSkillStatuses(proj.Manifest, proj.Lock, e.Verify(proj.Lock))
		scopes := groupByScope(rows)

		if treeJSON {
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(scopes)
		}

		printTree(cmd.OutOrStdout(), proj, scopes)
		return nil
	},
}

// groupByScope arranges the rows into the two scopes, dropping an empty one.
func groupByScope(rows []skillStatus) []treeScope {
	byScope := map[string][]treeSkill{}

	for _, row := range rows {
		byScope[row.Scope] = append(byScope[row.Scope], treeSkill{
			Name:    row.Name,
			Type:    row.Type,
			Status:  row.Status,
			Ref:     row.Ref,
			Source:  row.Source,
			Path:    row.Path,
			Targets: row.Targets,
		})
	}

	scopes := make([]treeScope, 0, len(byScope))
	// Project first: it is the common case, and the one the reader is usually
	// looking for.
	for _, scope := range []string{types.ScopeProject, types.ScopeGlobal} {
		skills, ok := byScope[scope]
		if !ok {
			continue
		}
		sort.SliceStable(skills, func(i, j int) bool { return skills[i].Name < skills[j].Name })
		scopes = append(scopes, treeScope{Scope: scope, Skills: skills})
	}

	return scopes
}

// printTree renders the layout.
func printTree(out io.Writer, proj *projectContext, scopes []treeScope) {
	if len(scopes) == 0 {
		_, _ = fmt.Fprintf(out, "No skills declared in %s\n", proj.ManifestPath)
		return
	}

	_, _ = fmt.Fprintln(out, proj.Root)

	for scopeIndex, scope := range scopes {
		lastScope := scopeIndex == len(scopes)-1
		_, _ = fmt.Fprintf(out, "%s %s (%s)\n",
			branch(lastScope), scope.Scope, canonicalRoot(scope.Scope, proj.Root))

		for skillIndex, skill := range scope.Skills {
			lastSkill := skillIndex == len(scope.Skills)-1
			_, _ = fmt.Fprintf(out, "%s%s %s\n",
				indent(lastScope), branch(lastSkill), describeTreeSkill(skill))

			for targetIndex, target := range skill.Targets {
				_, _ = fmt.Fprintf(out, "%s%s%s %s\n",
					indent(lastScope), indent(lastSkill),
					branch(targetIndex == len(skill.Targets)-1), target)
			}
		}
	}
}

// describeTreeSkill is the one-line label for a skill node.
func describeTreeSkill(skill treeSkill) string {
	label := fmt.Sprintf("%s [%s]", skill.Name, skill.Status)
	if skill.Ref != "" {
		label = fmt.Sprintf("%s [%s] %s", skill.Name, skill.Status, skill.Ref)
	}
	if skill.Type == types.TypeRule {
		label += " (rule)"
	}
	return label
}

// canonicalRoot names the directory a scope's skills live in.
func canonicalRoot(scope string, projectRoot string) string {
	return types.CanonicalDir(scope, projectRoot)
}

// branch is the connector for a node, given whether it is the last sibling.
func branch(last bool) string {
	if last {
		return "`--"
	}
	return "|--"
}

// indent is the continuation for a node's children.
func indent(lastParent bool) string {
	if lastParent {
		return "    "
	}
	return "|   "
}

func init() {
	treeCmd.Flags().BoolVar(&treeJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(treeCmd)
}
