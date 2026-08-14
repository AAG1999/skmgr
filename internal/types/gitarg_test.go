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

package types

import (
	"strings"
	"testing"
)

// git decides what an argument means from its first character, and options
// like --upload-pack take a command to run. A manifest is a file people copy
// from a README, so this has to be refused rather than merely fail to clone.
func TestValidateGitArgument_RejectsOptionLookingValues(t *testing.T) {
	hostile := []string{
		"--upload-pack=touch /tmp/pwned",
		"--config=core.pager=sh",
		"-c",
		"-",
		"--",
		"--exec=whatever",
		"-u",
	}

	for _, value := range hostile {
		err := ValidateGitArgument("source", value)
		if err == nil {
			t.Errorf("ValidateGitArgument(%q) = nil, want it refused", value)
			continue
		}
		if !strings.Contains(err.Error(), "option") {
			t.Errorf("the error for %q should explain why, got: %v", value, err)
		}
	}
}

// Anything a person would actually write must still go through, including the
// forms that contain a dash somewhere other than the front.
func TestValidateGitArgument_AcceptsRealValues(t *testing.T) {
	fine := []string{
		"",
		"https://github.com/owner/repo.git",
		"git@github.com:owner/repo.git",
		"ssh://git@internal.example/team/skills.git",
		"file:///srv/mirrors/skills.git",
		"./relative/path",
		"../sibling",
		"/absolute/path",
		"git::/srv/mirror",
		"main",
		"release-2",
		"feature/some-branch",
		"v1.2.3",
		"v1.2.3-rc.1",
		"^1.2.0",
		"0123456789abcdef0123456789abcdef01234567",
		"C:\\repos\\skills",
	}

	for _, value := range fine {
		if err := ValidateGitArgument("ref", value); err != nil {
			t.Errorf("ValidateGitArgument(%q) = %v, want it accepted", value, err)
		}
	}
}

// The message names which field is at fault, since a manifest has both a
// source and a ref.
func TestValidateGitArgument_NamesTheField(t *testing.T) {
	err := ValidateGitArgument("ref", "--exec=sh")
	if err == nil {
		t.Fatal("expected a rejection")
	}
	if !strings.Contains(err.Error(), "ref") {
		t.Errorf("the error should name the field, got: %v", err)
	}
}
