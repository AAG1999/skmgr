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
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/AAG1999/skmgr/internal/manifest"
	"github.com/AAG1999/skmgr/internal/provider"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	discoverRef     string
	discoverPath    string
	discoverOffline bool
)

// descriptionLimit is how much of an untrusted SKILL.md description reaches
// the agent. Longer text is data we do not need to pass through whole.
const descriptionLimit = 320

var discoverCmd = &cobra.Command{
	Use:   "discover <source>",
	Short: "List the skills and rules one source offers",
	Long: `Show what skmgr add would install from one git URL or local path.

discover uses the same lookup as install and writes only to the download
cache. It does not search the web and it does not change the project.`,
	Args: cobra.ExactArgs(1),
	RunE: runDiscover,
}

type discoverDocument struct {
	Source      string          `json:"source"`
	Ref         string          `json:"ref,omitempty"`
	ResolvedRef string          `json:"resolved_ref,omitempty"`
	CommitSHA   string          `json:"commit_sha,omitempty"`
	Entries     []discoverEntry `json:"entries"`
}

type discoverEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Path        string `json:"path"`
	Description string `json:"description,omitempty"`
	Truncated   bool   `json:"truncated,omitempty"`
}

func runDiscover(cmd *cobra.Command, args []string) error {
	source := args[0]
	if err := types.ValidateGitArgument("source", source); err != nil {
		return &UsageError{Msg: err.Error()}
	}
	if err := types.ValidateGitArgument("ref", discoverRef); err != nil {
		return &UsageError{Msg: err.Error()}
	}
	if strings.HasPrefix(discoverPath, "-") {
		return &UsageError{Msg: fmt.Sprintf("invalid argument %q for --path", discoverPath)}
	}

	result, err := provider.GetProvider(source).Fetch(types.SkillDependency{
		Source: source,
		Path:   discoverPath,
		Ref:    discoverRef,
	}, types.CacheDir(), provider.FetchOptions{Offline: offlineRequested(discoverOffline)})
	if err != nil {
		return err
	}

	entries := make([]discoverEntry, 0, len(result.SourceDirs))
	for _, dir := range result.SourceDirs {
		rel, err := filepath.Rel(result.Root, dir)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if rel == "." {
			rel = ""
		}
		name, err := manifest.DeriveName(source, rel)
		if err != nil {
			return err
		}
		kind := types.TypeRule
		if _, err := os.Stat(filepath.Join(dir, "SKILL.md")); err == nil {
			kind = types.TypeSkill
		}
		description, truncated := readDescription(dir)
		entries = append(entries, discoverEntry{
			Name:        name,
			Type:        kind,
			Path:        rel,
			Description: description,
			Truncated:   truncated,
		})
	}
	if len(entries) == 0 {
		return fmt.Errorf("no skills found in %s", source)
	}

	doc := discoverDocument{
		Source:      source,
		Ref:         discoverRef,
		ResolvedRef: result.ResolvedRef,
		CommitSHA:   result.CommitSHA,
		Entries:     entries,
	}
	if jsonOutput {
		return emit(cmd, doc)
	}

	out := cmd.OutOrStdout()
	if doc.CommitSHA != "" {
		_, _ = fmt.Fprintf(out, "commit %s\n", doc.CommitSHA)
	}
	if doc.ResolvedRef != "" {
		_, _ = fmt.Fprintf(out, "resolved ref %s\n", doc.ResolvedRef)
	}
	for _, entry := range entries {
		_, _ = fmt.Fprintf(out, "%s\t%s\t%s\t%s\n", entry.Name, entry.Type, entry.Path, entry.Description)
	}
	return nil
}

// readDescription returns the SKILL.md frontmatter description with control
// characters removed. The bool is true when the text was cut to descriptionLimit.
func readDescription(dir string) (string, bool) {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return "", false
	}
	text := string(data)
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return "", false
	}
	rest := text
	if strings.HasPrefix(rest, "---\r\n") {
		rest = rest[len("---\r\n"):]
	} else {
		rest = rest[len("---\n"):]
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return "", false
	}
	var raw string
	for _, line := range strings.Split(rest[:end], "\n") {
		trim := strings.TrimSpace(line)
		if value, ok := strings.CutPrefix(trim, "description:"); ok {
			raw = strings.Trim(strings.TrimSpace(value), `"'`)
			break
		}
	}
	if raw == "" {
		return "", false
	}
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, raw)
	if len([]rune(cleaned)) <= descriptionLimit {
		return cleaned, false
	}
	return string([]rune(cleaned)[:descriptionLimit]), true
}

func init() {
	discoverCmd.Flags().StringVar(&discoverRef, "ref", "", "Git branch, tag, commit, or version range")
	discoverCmd.Flags().StringVar(&discoverPath, "path", "", "Path or glob within the source")
	discoverCmd.Flags().BoolVar(&discoverOffline, "offline", false, "Never access the network; use only the local cache")
	jsonFlag(discoverCmd)
	rootCmd.AddCommand(discoverCmd)
}
