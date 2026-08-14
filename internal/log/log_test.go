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

package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestLogger_LevelFiltering(t *testing.T) {
	tests := []struct {
		level    Level
		wantSeen []string
		wantGone []string
	}{
		{
			level:    LevelDebug,
			wantSeen: []string{"a debug", "an info", "a warn", "an error"},
		},
		{
			level:    LevelInfo,
			wantSeen: []string{"an info", "a warn", "an error"},
			wantGone: []string{"a debug"},
		},
		{
			level:    LevelWarn,
			wantSeen: []string{"a warn", "an error"},
			wantGone: []string{"a debug", "an info"},
		},
		{
			level:    LevelError,
			wantSeen: []string{"an error"},
			wantGone: []string{"a debug", "an info", "a warn"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.level.String(), func(t *testing.T) {
			var buf bytes.Buffer
			l := New(&buf, tc.level)

			l.Debugf("a debug")
			l.Infof("an info")
			l.Warnf("a warn")
			l.Errorf("an error")

			got := buf.String()
			for _, want := range tc.wantSeen {
				if !strings.Contains(got, want) {
					t.Errorf("output is missing %q at level %s; got:\n%s", want, tc.level, got)
				}
			}
			for _, unwanted := range tc.wantGone {
				if strings.Contains(got, unwanted) {
					t.Errorf("output contains %q at level %s but should not; got:\n%s", unwanted, tc.level, got)
				}
			}
		})
	}
}

func TestLogger_Prefixes(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelDebug)

	l.Debugf("tracing")
	l.Infof("plain")
	l.Warnf("careful")
	l.Errorf("broken")

	got := buf.String()

	for _, want := range []string{"debug: tracing", "warning: careful", "error: broken"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q; got:\n%s", want, got)
		}
	}

	// Info is the normal channel and should not be decorated.
	if !strings.Contains(got, "\nplain\n") && !strings.HasPrefix(got, "plain\n") {
		t.Errorf("info message should be unprefixed; got:\n%s", got)
	}
}

func TestLogger_Formatting(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelInfo)

	l.Infof("installed %d skills into %s", 3, ".agents")

	if got := buf.String(); got != "installed 3 skills into .agents\n" {
		t.Errorf("output = %q, want %q", got, "installed 3 skills into .agents\n")
	}
}

func TestParseLevel(t *testing.T) {
	valid := map[string]Level{
		"debug":   LevelDebug,
		"info":    LevelInfo,
		"":        LevelInfo,
		"warn":    LevelWarn,
		"warning": LevelWarn,
		"error":   LevelError,
	}

	for input, want := range valid {
		got, err := ParseLevel(input)
		if err != nil {
			t.Errorf("ParseLevel(%q) returned error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseLevel(%q) = %v, want %v", input, got, want)
		}
	}

	if _, err := ParseLevel("chatty"); err == nil {
		t.Error("ParseLevel(\"chatty\") should have returned an error")
	}
}

func TestLogger_Enabled(t *testing.T) {
	l := New(&bytes.Buffer{}, LevelWarn)

	if l.Enabled(LevelInfo) {
		t.Error("Enabled(LevelInfo) = true at warn threshold, want false")
	}
	if !l.Enabled(LevelError) {
		t.Error("Enabled(LevelError) = false at warn threshold, want true")
	}
}

func TestLogger_SetLevel(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, LevelError)

	l.Infof("hidden")
	if buf.Len() != 0 {
		t.Fatalf("info should be suppressed at error level; got %q", buf.String())
	}

	l.SetLevel(LevelInfo)
	l.Infof("shown")
	if !strings.Contains(buf.String(), "shown") {
		t.Errorf("info should appear after lowering the level; got %q", buf.String())
	}
}
