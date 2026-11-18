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

package workflow

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

// templateDigests records the hash of the embedded templates for each
// TemplateRevision. Changing the text without bumping the revision fails.
var templateDigests = map[int]string{
	1: "1337998185e32525b58f5444f9e857d41ae44ef2d41d1ece21931c451bf6f2de",
}

func TestGolden_WorkflowFiles(t *testing.T) {
	root := t.TempDir()
	for _, name := range types.SupportedAgentNames() {
		files, err := Render(types.DefaultAgents()[name], root)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			golden := filepath.Join("testdata", "workflow", filepath.FromSlash(file.Rel))
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll(filepath.Dir(golden), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, []byte(file.Content), 0644); err != nil {
					t.Fatal(err)
				}
				continue
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("reading %s: %v (set UPDATE_GOLDEN=1 to create it)", golden, err)
			}
			if lf(string(want)) != lf(file.Content) {
				t.Errorf("%s does not match the golden file", file.Rel)
			}
		}
	}
}

func TestTemplateRevisionTracksText(t *testing.T) {
	names := make([]string, 0, len(TemplateSources()))
	for name := range TemplateSources() {
		names = append(names, name)
	}
	sort.Strings(names)
	sum := sha256.New()
	for _, name := range names {
		sum.Write([]byte(name))
		sum.Write([]byte(TemplateSources()[name]))
	}
	got := hex.EncodeToString(sum.Sum(nil))
	want, ok := templateDigests[TemplateRevision]
	if !ok || want != got {
		t.Fatalf("template text hash %s is not recorded for revision %d; bump TemplateRevision and record the hash", got, TemplateRevision)
	}
}
