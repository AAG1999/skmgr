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
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// catalog builds a source tree holding the given number of skills, buried under
// the given number of unrelated directories.
//
// The noise matters: auto-discovery walks everything, so a monorepo where the
// skills are a small corner of a large tree is the case that actually costs
// something.
func catalog(b *testing.B, skills int, noiseDirs int) string {
	b.Helper()

	root := b.TempDir()

	for i := 0; i < skills; i++ {
		dir := filepath.Join(root, "skills", fmt.Sprintf("skill-%04d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("# skill\n"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	for i := 0; i < noiseDirs; i++ {
		dir := filepath.Join(root, "src", fmt.Sprintf("pkg%03d", i%50), fmt.Sprintf("sub%03d", i))
		if err := os.MkdirAll(dir, 0755); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "code.go"), []byte("package p\n"), 0644); err != nil {
			b.Fatal(err)
		}
	}

	return root
}

// BenchmarkAutoDiscovery measures walking a source tree with no path given.
func BenchmarkAutoDiscovery(b *testing.B) {
	shapes := []struct {
		name   string
		skills int
		noise  int
	}{
		{"20_skills_no_noise", 20, 0},
		{"20_skills_in_a_monorepo", 20, 2000},
		{"200_skills", 200, 500},
	}

	for _, shape := range shapes {
		b.Run(shape.name, func(b *testing.B) {
			root := catalog(b, shape.skills, shape.noise)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				collector := &skillCollector{requireConventionalParent: true}
				if err := collector.walk(root); err != nil {
					b.Fatal(err)
				}
				if len(collector.dirs) != shape.skills {
					b.Fatalf("found %d skills, want %d", len(collector.dirs), shape.skills)
				}
			}
		})
	}
}

// BenchmarkWildcardDiscovery measures the glob path, which is what most
// monorepo manifests actually use.
func BenchmarkWildcardDiscovery(b *testing.B) {
	for _, skills := range []int{20, 200} {
		b.Run(fmt.Sprintf("%d_skills", skills), func(b *testing.B) {
			root := catalog(b, skills, 500)
			provider := &LocalProvider{}
			dest := b.TempDir()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := provider.Fetch(
					types.SkillDependency{Name: "bench", Source: root, Path: "skills/*"},
					dest,
					FetchOptions{},
				)
				if err != nil {
					b.Fatal(err)
				}
				if len(result.SourceDirs) != skills {
					b.Fatalf("matched %d skills, want %d", len(result.SourceDirs), skills)
				}
			}
		})
	}
}

// taggedRepo builds a git repository carrying the given number of release
// tags, all on one commit so setup does not dominate the measurement.
func taggedRepo(b *testing.B, count int) string {
	b.Helper()

	dir := b.TempDir()
	skill := filepath.Join(dir, "skills", "versioned")
	if err := os.MkdirAll(skill, 0755); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skill, "SKILL.md"), []byte("# skill\n"), 0644); err != nil {
		b.Fatal(err)
	}

	run := func(args ...string) {
		b.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=bench", "GIT_AUTHOR_EMAIL=bench@example.com",
			"GIT_COMMITTER_NAME=bench", "GIT_COMMITTER_EMAIL=bench@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			b.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	run("init", "--quiet", "-b", "main")
	run("add", ".")
	run("commit", "--quiet", "-m", "initial")

	for i := 0; i < count; i++ {
		run("tag", fmt.Sprintf("v%d.%d.%d", i/100, (i/10)%10, i%10))
	}

	return dir
}

// BenchmarkListTags measures reading a repository's tags, which runs once per
// source whenever a version range has to be resolved.
func BenchmarkListTags(b *testing.B) {
	for _, count := range []int{10, 200} {
		b.Run(fmt.Sprintf("%d_tags", count), func(b *testing.B) {
			repo := taggedRepo(b, count)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				tags, err := listTags(repo)
				if err != nil {
					b.Fatal(err)
				}
				if len(tags) != count {
					b.Fatalf("read %d tags, want %d", len(tags), count)
				}
			}
		})
	}
}
