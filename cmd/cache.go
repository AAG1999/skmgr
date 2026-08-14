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
	"strings"
	"time"

	"github.com/AAG1999/skmgr/internal/provider"
	"github.com/AAG1999/skmgr/internal/types"
	"github.com/spf13/cobra"
)

var (
	pruneOlderThan string
	pruneMaxSize   string
	pruneDryRun    bool
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Inspect and prune the download cache",
	Long: `Manage the shared download cache at ~/.skmgr/cache.

Cached content is keyed by commit, so following a moving branch accumulates one
tree per commit it ever resolved to. Pruning removes the ones you no longer use.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return cmd.Help()
	},
}

var cachePathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the cache directory",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintln(cmd.OutOrStdout(), types.CacheDir())
		return err
	},
}

var cacheInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show what the cache is holding",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		repos, err := provider.ScanRepos(types.CacheDir())
		if err != nil {
			return fmt.Errorf("scanning the cache: %w", err)
		}

		out := cmd.OutOrStdout()

		if len(repos) == 0 {
			_, err := fmt.Fprintf(out, "Cache is empty (%s)\n", types.CacheDir())
			return err
		}

		var commits int
		var treeBytes, cloneBytes int64
		for _, repo := range repos {
			commits += len(repo.Commits)
			cloneBytes += repo.CloneSize
			for _, commit := range repo.Commits {
				treeBytes += commit.Size
			}
		}

		// The clone total is broken out because it is often the larger half,
		// and because it explains why pruning trees alone frees less than the
		// commit count suggests.
		_, err = fmt.Fprintf(out, "%s\n%s, %s, %s (%s in clones)\n",
			types.CacheDir(),
			plural(len(repos), "repository", "repositories"),
			plural(commits, "cached commit", "cached commits"),
			humanSize(treeBytes+cloneBytes), humanSize(cloneBytes))
		return err
	},
}

var cachePruneCmd = &cobra.Command{
	Use:   "prune",
	Short: "Remove cached content",
	Long: `Remove cached commit trees.

With no flags this empties the cache. Use --older-than to drop what you have not
used recently, or --max-size to keep the cache under a budget by removing the
least recently used entries first.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := provider.PruneOptions{DryRun: pruneDryRun}

		if pruneOlderThan != "" {
			d, err := parseDuration(pruneOlderThan)
			if err != nil {
				return err
			}
			opts.OlderThan = d
		}

		if pruneMaxSize != "" {
			size, err := parseSize(pruneMaxSize)
			if err != nil {
				return err
			}
			opts.MaxSize = size
		}

		result, err := provider.PruneCache(types.CacheDir(), opts)
		if err != nil {
			return fmt.Errorf("pruning the cache: %w", err)
		}

		out := cmd.OutOrStdout()

		verb := "Removed"
		if pruneDryRun {
			verb = "Would remove"
		}

		if len(result.Removed) == 0 && result.RemovedRepos == 0 {
			_, err := fmt.Fprintln(out, "Nothing to prune")
			return err
		}

		repos := ""
		if result.RemovedRepos > 0 {
			repos = fmt.Sprintf(" and %s",
				plural(result.RemovedRepos, "clone", "clones"))
		}

		_, err = fmt.Fprintf(out, "%s %s%s, freeing %s (%s remaining)\n",
			verb,
			plural(len(result.Removed), "cached commit", "cached commits"),
			repos,
			humanSize(result.FreedSize), humanSize(result.Remaining))
		return err
	},
}

// plural renders a count with the right noun form.
func plural(n int, one string, many string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, one)
	}
	return fmt.Sprintf("%d %s", n, many)
}

// parseDuration accepts Go durations plus day and week suffixes, because "keep
// two weeks of cache" is the natural way to think about this and "336h" is not.
func parseDuration(value string) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)

	for suffix, unit := range map[string]time.Duration{
		"d": 24 * time.Hour,
		"w": 7 * 24 * time.Hour,
	} {
		if !strings.HasSuffix(trimmed, suffix) {
			continue
		}
		var count float64
		if _, err := fmt.Sscanf(strings.TrimSuffix(trimmed, suffix), "%f", &count); err != nil {
			return 0, fmt.Errorf("invalid duration %q: use forms like 30d, 2w, or 48h", value)
		}
		return time.Duration(count * float64(unit)), nil
	}

	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, fmt.Errorf("invalid duration %q: use forms like 30d, 2w, or 48h", value)
	}
	return d, nil
}

// parseSize accepts plain bytes or a KB/MB/GB suffix.
func parseSize(value string) (int64, error) {
	trimmed := strings.ToUpper(strings.TrimSpace(value))

	units := []struct {
		suffix string
		factor int64
	}{
		{"GB", 1 << 30},
		{"G", 1 << 30},
		{"MB", 1 << 20},
		{"M", 1 << 20},
		{"KB", 1 << 10},
		{"K", 1 << 10},
		{"B", 1},
	}

	for _, unit := range units {
		if !strings.HasSuffix(trimmed, unit.suffix) {
			continue
		}
		var count float64
		if _, err := fmt.Sscanf(strings.TrimSuffix(trimmed, unit.suffix), "%f", &count); err != nil {
			return 0, fmt.Errorf("invalid size %q: use forms like 500MB or 2GB", value)
		}
		return int64(count * float64(unit.factor)), nil
	}

	var count int64
	if _, err := fmt.Sscanf(trimmed, "%d", &count); err != nil {
		return 0, fmt.Errorf("invalid size %q: use forms like 500MB or 2GB", value)
	}
	return count, nil
}

// humanSize formats a byte count for people.
func humanSize(bytes int64) string {
	const unit = 1024

	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}

	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func init() {
	cachePruneCmd.Flags().StringVar(&pruneOlderThan, "older-than", "",
		"Remove entries not used within this period (e.g. 30d, 2w, 48h)")
	cachePruneCmd.Flags().StringVar(&pruneMaxSize, "max-size", "",
		"Keep the cache under this size, dropping least recently used first (e.g. 500MB)")
	cachePruneCmd.Flags().BoolVar(&pruneDryRun, "dry-run", false,
		"Report what would be removed without removing it")

	cacheCmd.AddCommand(cachePathCmd)
	cacheCmd.AddCommand(cacheInfoCmd)
	cacheCmd.AddCommand(cachePruneCmd)
	rootCmd.AddCommand(cacheCmd)
}
