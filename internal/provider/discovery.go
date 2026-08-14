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
	"fmt"
	"os"
	"path/filepath"

	"github.com/AAG1999/skmgr/internal/log"
)

// skillCollector gathers the directories a source offers as skills or rules.
//
// It exists so the git and local providers apply one set of rules. They used to
// carry a copy each, which is how they drifted into warning through different
// channels for the same condition.
type skillCollector struct {
	dirs []string

	// requireConventionalParent limits matches to a directory sitting directly
	// inside skills/ or rules/.
	//
	// This is only right when walking a whole tree looking for anything that
	// might be a skill: without it a SKILL.md quoted inside a docs/ folder or a
	// test fixture would be installed as though it were real. When the user
	// wrote the path themselves it is wrong, because they have already said
	// where to look and a warning that silently drops their match reads as
	// skmgr ignoring them.
	requireConventionalParent bool
}

// consider adds a directory if it declares itself a skill or a rule.
func (c *skillCollector) consider(path string) {
	_, skill := statMarker(path, "SKILL.md")
	_, rule := statMarker(path, "AGENTS.md")

	if !skill && !rule {
		return
	}

	if c.requireConventionalParent {
		parent := filepath.Base(filepath.Dir(path))
		if skill && parent != "skills" {
			log.Debugf("skipping SKILL.md in %s during auto-discovery (parent directory is not 'skills')", path)
			return
		}
		if rule && parent != "rules" {
			log.Debugf("skipping AGENTS.md in %s during auto-discovery (parent directory is not 'rules')", path)
			return
		}
	}

	c.dirs = append(c.dirs, path)
}

// walk collects every skill directory under root, skipping the git metadata.
func (c *skillCollector) walk(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if d.Name() == ".git" {
			return filepath.SkipDir
		}

		c.consider(path)
		return nil
	})
}

// considerMatches adds every glob match that is a directory holding a skill.
func (c *skillCollector) considerMatches(matches []string) {
	for _, match := range matches {
		info, err := os.Stat(match)
		if err != nil || !info.IsDir() {
			continue
		}
		c.consider(match)
	}
}

// statMarker reports whether a marker file exists in a directory.
func statMarker(dir string, name string) (string, bool) {
	path := filepath.Join(dir, name)
	if _, err := os.Stat(path); err != nil {
		return "", false
	}
	return path, true
}

// rootFallback decides what to install when auto-discovery found no
// skills/ or rules/ directory.
//
// A source whose root holds a SKILL.md or AGENTS.md is a single-skill
// repository, and installing the root is exactly right. Without one of those
// files there is nothing identifying the source as a skill at all, and copying
// the whole tree would install a README, a build system, and whatever else
// happens to be in the repository. That silently produces a broken install the
// user only notices when their agent ignores it, so it is refused instead.
func rootFallback(root string, describe string) ([]string, error) {
	if hasSkillMarker(root) {
		return []string{root}, nil
	}

	return nil, fmt.Errorf(
		"no skills found in %s: expected a skills/ or rules/ directory, "+
			"or a SKILL.md or AGENTS.md at the root. "+
			"Set `path:` in skmgr.yml to point at the directory holding the skill",
		describe,
	)
}

// hasSkillMarker reports whether a directory declares itself a skill or a rule.
func hasSkillMarker(dir string) bool {
	for _, marker := range []string{"SKILL.md", "AGENTS.md"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}
