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
	"fmt"
	"slices"
	"strings"

	"github.com/AAG1999/skmgr/internal/engine"
	"github.com/AAG1999/skmgr/internal/lockfile"
	"github.com/AAG1999/skmgr/internal/log"
	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	addName    string
	addPath    string
	addRef     string
	addType    string
	addScope   string
	addTargets []string
	addDryRun  bool
)

var addCmd = &cobra.Command{
	Use:   "add <source>",
	Short: "Add a skill/rule to the manifest and install it",
	Long: `Add a dependency to the manifest and install it immediately.

The name is derived from the path (or the repository name) unless --name is
given, and is only written to the manifest when it differs from what would be
derived.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		source := args[0]

		proj, err := loadProject()
		if err != nil {
			return fmt.Errorf("%w (run 'skmgr init' first?)", err)
		}

		skillType, err := resolveSkillType(addType)
		if err != nil {
			return err
		}

		scope, err := resolveSkillScope(addScope, globalScope)
		if err != nil {
			return err
		}

		// Work out the effective name so duplicates are caught before install,
		// and so we know whether the manifest needs an explicit name at all.
		name := addName
		if name == "" {
			if manifest.IsWildcard(addPath) {
				// Wildcards expand into many skills, each named at resolve time.
				name = ""
			} else {
				derived, err := manifest.DeriveName(source, addPath)
				if err != nil {
					return fmt.Errorf("%w (pass --name)", err)
				}
				name = derived
			}
		}

		if name != "" {
			if err := types.ValidateName(name); err != nil {
				return err
			}
			for _, s := range proj.Manifest.Skills {
				if s.Name == name {
					return fmt.Errorf("skill %q already exists in manifest", name)
				}
			}
		}

		newSkill := types.SkillDependency{
			Source:  source,
			Path:    addPath,
			Ref:     addRef,
			Type:    skillType,
			Scope:   scope,
			Targets: addTargets,
		}

		// Only record the name when it is not what derivation would produce.
		// Keeping the manifest free of redundant aliases is the point of A5.
		if name != "" && !nameIsDerivable(source, addPath, name) {
			newSkill.Name = name
		}

		// The engine needs a concrete name even when the manifest omits it.
		installSkill := newSkill
		installSkill.Name = name

		updated := *proj.Manifest
		updated.Skills = append(slices.Clone(proj.Manifest.Skills), installSkill)

		if addDryRun {
			return runDryRun(cmd, proj, &updated, engine.SyncOptions{})
		}

		e := engine.NewEngine(proj.Root, types.CacheDir())

		newLock, err := e.Sync(&updated, proj.Lock, engine.SyncOptions{})
		if err != nil {
			return fmt.Errorf("failed to install newly added skill: %w", err)
		}

		// Persist only after a successful install, so a failure leaves the
		// manifest untouched.
		toWrite := *proj.Manifest
		toWrite.Skills = append(slices.Clone(proj.Manifest.Skills), newSkill)

		if err := manifest.Write(proj.ManifestPath, &toWrite); err != nil {
			return err
		}
		if err := lockfile.Write(proj.LockPath, newLock); err != nil {
			return err
		}

		label := name
		if label == "" {
			label = addPath
		}
		log.Infof("Added and installed %s %q", skillType, label)
		return nil
	},
}

// nameIsDerivable reports whether derivation would produce exactly this name,
// in which case writing it to the manifest would be redundant.
func nameIsDerivable(source, path, name string) bool {
	derived, err := manifest.DeriveName(source, path)
	return err == nil && derived == name
}

// resolveSkillType validates --type instead of silently falling back to the
// default, which used to turn a typo into the wrong dependency kind.
func resolveSkillType(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return types.TypeSkill, nil
	}
	if !slices.Contains(types.ValidTypes(), value) {
		return "", fmt.Errorf("invalid --type %q (valid: %s)", value, strings.Join(types.ValidTypes(), ", "))
	}
	return value, nil
}

// resolveSkillScope validates --scope, honoring the global flag as an override.
func resolveSkillScope(value string, global bool) (string, error) {
	if global {
		return types.ScopeGlobal, nil
	}
	if strings.TrimSpace(value) == "" {
		return types.ScopeProject, nil
	}
	if !slices.Contains(types.ValidScopes(), value) {
		return "", fmt.Errorf("invalid --scope %q (valid: %s)", value, strings.Join(types.ValidScopes(), ", "))
	}
	return value, nil
}

func init() {
	addCmd.Flags().StringVarP(&addName, "name", "n", "", "Local alias (derived from path or source if omitted)")
	addCmd.Flags().StringVarP(&addPath, "path", "p", "", "Path within the repository")
	addCmd.Flags().StringVar(&addRef, "ref", "", "Git branch, tag, or commit SHA (default: the repository's default branch)")
	addCmd.Flags().StringVarP(&addType, "type", "t", "skill", "Type of dependency ('skill' or 'rule')")
	addCmd.Flags().StringVarP(&addScope, "scope", "s", "project", "Installation scope ('project' or 'global')")
	addCmd.Flags().StringSliceVar(&addTargets, "targets", nil, "Specific agent targets (overrides manifest global targets)")
	addGlobalFlag(addCmd, "Shorthand for --scope global (installs to ~/.agents/)")
	dryRunFlag(addCmd, &addDryRun, "adding this skill")

	rootCmd.AddCommand(addCmd)
}
