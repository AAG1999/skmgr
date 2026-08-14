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

package linker

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/AAG1999/skmgr/internal/util/atomicfile"
)

func getDelimiters(name string) (string, string) {
	start := fmt.Sprintf("<!-- skmgr:start:%s -->", name)
	end := fmt.Sprintf("<!-- skmgr:end:%s -->", name)
	return start, end
}

// checkBalancedDelimiters verifies every start delimiter has a matching end.
// Rewriting a file whose markers are unbalanced would silently discard
// everything after the orphaned start marker, so callers must refuse instead.
func checkBalancedDelimiters(targetPath string, lines []string, startDelim, endDelim string) error {
	depth := 0
	for i, line := range lines {
		switch strings.TrimSpace(line) {
		case startDelim:
			if depth > 0 {
				return fmt.Errorf(
					"%s: found a second %q at line %d before the previous section was closed; "+
						"repair the file by hand and re-run", targetPath, startDelim, i+1)
			}
			depth++
		case endDelim:
			if depth == 0 {
				return fmt.Errorf(
					"%s: found %q at line %d without a matching start marker; "+
						"repair the file by hand and re-run", targetPath, endDelim, i+1)
			}
			depth--
		}
	}
	if depth != 0 {
		return fmt.Errorf(
			"%s: %q is missing its closing %q; refusing to rewrite the file so your content is not lost",
			targetPath, startDelim, endDelim)
	}
	return nil
}

// mergeSection inserts or updates a delimited section in the target file.
func mergeSection(targetPath string, sectionName string, content string) error {
	startDelim, endDelim := getDelimiters(sectionName)

	var lines []string
	if data, err := os.ReadFile(targetPath); err == nil {
		lines = strings.Split(string(data), "\n")
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("reading target file: %w", err)
	}

	if err := checkBalancedDelimiters(targetPath, lines, startDelim, endDelim); err != nil {
		return err
	}

	var newLines []string
	inSection := false
	sectionFound := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == startDelim {
			inSection = true
			sectionFound = true
			// Inject new content
			newLines = append(newLines, startDelim)
			newLines = append(newLines, strings.TrimSpace(content))
			newLines = append(newLines, endDelim)
			continue
		}

		if trimmed == endDelim {
			inSection = false
			continue
		}

		if !inSection {
			newLines = append(newLines, line)
		}
	}

	// If the section didn't exist, append it at the end
	if !sectionFound {
		if len(newLines) > 0 && newLines[len(newLines)-1] != "" {
			newLines = append(newLines, "")
		}
		newLines = append(newLines, startDelim)
		newLines = append(newLines, strings.TrimSpace(content))
		newLines = append(newLines, endDelim)
	}

	// Ensure parent directories exist
	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("creating parent directories for merge: %w", err)
	}

	// Join and write
	output := strings.Join(newLines, "\n")
	if !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if err := atomicfile.Write(targetPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing merged file: %w", err)
	}

	return nil
}

// managedSectionStart matches the opening marker of any skmgr-managed section,
// capturing the section name. Used when clearing every block at once, where the
// names are not known up front.
var managedSectionStart = regexp.MustCompile(`^<!--\s*skmgr:start:(.+?)\s*-->$`)

// managedSectionNames lists every managed section present in a file, in the
// order they appear.
func managedSectionNames(lines []string) []string {
	var names []string
	for _, line := range lines {
		if m := managedSectionStart.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
			names = append(names, m[1])
		}
	}
	return names
}

// removeAllSections strips every skmgr-managed block from the target file,
// leaving the user's own content in place. Used by `skmgr clean`, which would
// otherwise leave orphaned blocks pointing at skills that no longer exist.
func removeAllSections(targetPath string) error {
	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading target file: %w", err)
	}

	for _, name := range managedSectionNames(strings.Split(string(data), "\n")) {
		if err := removeSection(targetPath, name); err != nil {
			return err
		}
	}

	return nil
}

// removeSection completely removes a delimited section from the target file.
func removeSection(targetPath string, sectionName string) error {
	startDelim, endDelim := getDelimiters(sectionName)

	data, err := os.ReadFile(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to remove
		}
		return fmt.Errorf("reading target file: %w", err)
	}

	lines := strings.Split(string(data), "\n")

	if err := checkBalancedDelimiters(targetPath, lines, startDelim, endDelim); err != nil {
		return err
	}

	var newLines []string
	inSection := false
	removedSomething := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == startDelim {
			inSection = true
			removedSomething = true
			continue
		}

		if trimmed == endDelim {
			inSection = false
			continue
		}

		if !inSection {
			newLines = append(newLines, line)
		}
	}

	if !removedSomething {
		return nil
	}

	// Clean up trailing newlines left by the removal
	for len(newLines) > 0 && newLines[len(newLines)-1] == "" {
		newLines = newLines[:len(newLines)-1]
	}

	output := strings.Join(newLines, "\n")
	if output != "" && !strings.HasSuffix(output, "\n") {
		output += "\n"
	}

	if err := atomicfile.Write(targetPath, []byte(output), 0644); err != nil {
		return fmt.Errorf("writing file after removal: %w", err)
	}

	return nil
}
