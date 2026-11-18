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
	"errors"
	"testing"

	"github.com/AAG1999/skmgr/internal/types"
)

func TestFetch_UnreachableRemoteIsNetworkError(t *testing.T) {
	previous := gitRetryAttempts
	gitRetryAttempts = 1
	t.Cleanup(func() { gitRetryAttempts = previous })

	_, err := (&GitProvider{}).Fetch(types.SkillDependency{
		Source: "https://127.0.0.1:1/repo.git",
	}, t.TempDir(), FetchOptions{})
	var net *NetworkError
	if !errors.As(err, &net) {
		t.Fatalf("got %v, want NetworkError", err)
	}
}

func TestFetch_OfflineCacheMissIsNetworkError(t *testing.T) {
	_, err := (&GitProvider{}).Fetch(types.SkillDependency{
		Source: "https://127.0.0.1:1/never-cached.git",
	}, t.TempDir(), FetchOptions{Offline: true})
	var net *NetworkError
	if !errors.As(err, &net) {
		t.Fatalf("got %v, want NetworkError", err)
	}
	if net.Op != "offline" {
		t.Fatalf("op = %s, want offline", net.Op)
	}
}
