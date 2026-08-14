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

package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// isolate points every input at a temp directory and clears the cached file,
// so one test cannot observe another's config or the developer's real one.
func isolate(t *testing.T) string {
	t.Helper()

	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	// os.UserConfigDir reads a different variable on each platform, and every
	// one of them has to be redirected or the test reads the real config.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData", "Roaming"))

	for _, env := range []string{EnvConfig, EnvHome, EnvCacheDir, EnvOffline, EnvLogLevel} {
		t.Setenv(env, "")
	}

	Reset()
	t.Cleanup(Reset)

	return home
}

// abs is the absolute form of a path written in a config file or environment
// variable, which is what Load stores. Test paths are written Unix-style; on
// Windows an absolute path also carries a drive letter, so the expectation has
// to go through the same normalization rather than hardcode the input.
func abs(t *testing.T, path string) string {
	t.Helper()

	absolute, err := filepath.Abs(path)
	if err != nil {
		t.Fatalf("resolving %q: %v", path, err)
	}
	return absolute
}

// writeConfig writes a config file and points SKMGR_CONFIG at it.
func writeConfig(t *testing.T, contents string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
		t.Fatal(err)
	}

	t.Setenv(EnvConfig, path)
	Reset()

	return path
}

func TestLoad_DefaultsWithoutAFile(t *testing.T) {
	home := isolate(t)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	wantHome := filepath.Join(home, ".skmgr")
	if cfg.Home != wantHome {
		t.Errorf("Home = %q, want %q", cfg.Home, wantHome)
	}
	if want := filepath.Join(wantHome, "cache"); cfg.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, want)
	}
	if cfg.Offline {
		t.Error("Offline should default to false")
	}
	if cfg.Path != "" {
		t.Errorf("Path = %q, want empty when there is no file", cfg.Path)
	}
}

func TestLoad_ReadsTheConfigFile(t *testing.T) {
	isolate(t)
	path := writeConfig(t, `home: /srv/skmgr
cache_dir: /mnt/fast/skmgr-cache
offline: true
log_level: debug
targets:
  - cursor
  - claude-code
`)

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if cfg.Path != path {
		t.Errorf("Path = %q, want %q", cfg.Path, path)
	}
	if want := abs(t, "/srv/skmgr"); cfg.Home != want {
		t.Errorf("Home = %q, want %q", cfg.Home, want)
	}
	if want := abs(t, "/mnt/fast/skmgr-cache"); cfg.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, want)
	}
	if !cfg.Offline {
		t.Error("Offline should be true")
	}
	if cfg.LogLevel != "debug" {
		t.Errorf("LogLevel = %q", cfg.LogLevel)
	}
	if strings.Join(cfg.Targets, ",") != "cursor,claude-code" {
		t.Errorf("Targets = %v", cfg.Targets)
	}
}

// The environment is the more specific of the two, so it has to win.
func TestLoad_EnvironmentBeatsTheFile(t *testing.T) {
	isolate(t)
	writeConfig(t, `home: /from/file
cache_dir: /from/file/cache
offline: true
log_level: warn
`)

	t.Setenv(EnvHome, "/from/env")
	t.Setenv(EnvCacheDir, "/from/env/cache")
	t.Setenv(EnvOffline, "0")
	t.Setenv(EnvLogLevel, "error")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if want := abs(t, "/from/env"); cfg.Home != want {
		t.Errorf("Home = %q, want %q from the environment", cfg.Home, want)
	}
	if want := abs(t, "/from/env/cache"); cfg.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q from the environment", cfg.CacheDir, want)
	}
	if cfg.Offline {
		t.Error("SKMGR_OFFLINE=0 should override offline: true in the file")
	}
	if cfg.LogLevel != "error" {
		t.Errorf("LogLevel = %q, want the environment to win", cfg.LogLevel)
	}
}

// Relocating the state directory must carry the cache with it, unless the
// cache was relocated separately.
func TestLoad_CacheFollowsHome(t *testing.T) {
	isolate(t)
	t.Setenv(EnvHome, "/custom/state")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if want := abs(t, "/custom/state/cache"); cfg.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, want)
	}

	t.Setenv(EnvCacheDir, "/elsewhere")
	cfg, err = Load()
	if err != nil {
		t.Fatal(err)
	}
	if want := abs(t, "/elsewhere"); cfg.CacheDir != want {
		t.Errorf("CacheDir = %q, want the explicit value %q", cfg.CacheDir, want)
	}
	if want := abs(t, "/custom/state"); cfg.Home != want {
		t.Errorf("Home = %q, relocating the cache should not move the state from %q", cfg.Home, want)
	}
}

// A relative path would resolve against whatever directory the command ran in,
// so `skmgr install` here and `skmgr install` one level down would use two
// different caches.
func TestLoad_MakesPathsAbsolute(t *testing.T) {
	isolate(t)

	work := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(work); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })

	t.Setenv(EnvHome, "state")
	t.Setenv(EnvCacheDir, "downloads")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if !filepath.IsAbs(cfg.Home) {
		t.Errorf("Home = %q, want an absolute path", cfg.Home)
	}
	if !filepath.IsAbs(cfg.CacheDir) {
		t.Errorf("CacheDir = %q, want an absolute path", cfg.CacheDir)
	}
	if filepath.Base(cfg.Home) != "state" {
		t.Errorf("Home = %q, should still end in the configured directory", cfg.Home)
	}
}

// A config file is hand-written, and a person writing a path by hand writes
// the one their shell would have expanded for them.
func TestLoad_ExpandsATilde(t *testing.T) {
	home := isolate(t)
	writeConfig(t, "cache_dir: ~/skmgr-cache\n")

	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}

	if want := filepath.Join(home, "skmgr-cache"); cfg.CacheDir != want {
		t.Errorf("CacheDir = %q, want %q", cfg.CacheDir, want)
	}
}

// Silently ignoring a config file the user wrote would leave them staring at
// settings that never took effect.
func TestLoad_RejectsAMalformedFile(t *testing.T) {
	isolate(t)
	writeConfig(t, "home: [this is not a string\n")

	if _, err := Load(); err == nil {
		t.Fatal("expected an error for a malformed config file")
	}
}

func TestLoad_RejectsUnknownKeys(t *testing.T) {
	isolate(t)
	writeConfig(t, "cach_dir: /typo\n")

	_, err := Load()
	if err == nil {
		t.Fatal("expected an error for an unknown key, since a typo would otherwise be silent")
	}
	if !strings.Contains(err.Error(), "cach_dir") {
		t.Errorf("the error should name the unknown key, got: %v", err)
	}
}

func TestLoad_EmptyFileIsFine(t *testing.T) {
	isolate(t)
	writeConfig(t, "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("an empty config file should be valid: %v", err)
	}
	if cfg.Home == "" {
		t.Error("defaults should still apply")
	}
}

func TestLoad_MissingFileIsNotAnError(t *testing.T) {
	isolate(t)
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "nope.yml"))
	Reset()

	if _, err := Load(); err != nil {
		t.Fatalf("a missing config file should be fine: %v", err)
	}
}

func TestDefaultPath_HonorsTheEnvironment(t *testing.T) {
	isolate(t)
	t.Setenv(EnvConfig, "/explicit/config.yml")

	if got := DefaultPath(); got != "/explicit/config.yml" {
		t.Errorf("DefaultPath() = %q", got)
	}
}

// The config file goes where the platform puts config, which docs/reference/
// config.md documents per platform. Only Linux uses the XDG location.
func TestDefaultPath_UsesThePlatformConfigDirectory(t *testing.T) {
	home := isolate(t)

	var want string
	switch runtime.GOOS {
	case "darwin":
		want = filepath.Join(home, "Library", "Application Support", "skmgr", "config.yml")
	case "windows":
		want = filepath.Join(home, "AppData", "Roaming", "skmgr", "config.yml")
	default:
		want = filepath.Join(home, ".config", "skmgr", "config.yml")
	}

	if got := DefaultPath(); got != want {
		t.Errorf("DefaultPath() = %q, want %q", got, want)
	}
}

// The file is cached, but the environment must stay live: a process that
// changes HOME expects paths to follow.
func TestCurrent_TracksEnvironmentChanges(t *testing.T) {
	isolate(t)

	t.Setenv(EnvHome, "/first")
	if got, want := Current().Home, abs(t, "/first"); got != want {
		t.Fatalf("Home = %q, want %q", got, want)
	}

	t.Setenv(EnvHome, "/second")
	if got, want := Current().Home, abs(t, "/second"); got != want {
		t.Errorf("Home = %q, want %q without needing a reset", got, want)
	}
}

func TestCurrent_FallsBackToDefaultsOnError(t *testing.T) {
	isolate(t)
	writeConfig(t, "home: [broken\n")

	// Paths still have to resolve, so a broken file cannot leave them empty.
	if Current().Home == "" {
		t.Error("Current() should fall back to defaults")
	}
	if Err() == nil {
		t.Error("Err() should report the failure so the CLI can surface it")
	}
}

func TestTruthy(t *testing.T) {
	cases := map[string]bool{
		"":      false,
		"0":     false,
		"false": false,
		"no":    false,
		"off":   false,
		"  ":    false,
		"1":     true,
		"true":  true,
		"yes":   true,
		"on":    true,
		"any":   true,
	}

	for value, want := range cases {
		if got := truthy(value); got != want {
			t.Errorf("truthy(%q) = %v, want %v", value, got, want)
		}
	}
}
