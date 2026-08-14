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

// Package config resolves user-level settings from a config file, environment
// variables, and built-in defaults.
//
// Precedence runs from most to least specific: a command-line flag beats an
// environment variable, which beats the config file, which beats the default.
// Flags are applied by the commands themselves; everything below that line is
// resolved here so there is one place to look when a setting does not take.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	yaml "gopkg.in/yaml.v3"
)

// Environment variables recognized by skmgr.
const (
	// EnvConfig points at a config file, replacing the default location.
	EnvConfig = "SKMGR_CONFIG"

	// EnvHome relocates skmgr's state directory (cache, global ledger).
	EnvHome = "SKMGR_HOME"

	// EnvCacheDir relocates only the download cache, which is the part worth
	// putting on a different disk or sharing between CI jobs.
	EnvCacheDir = "SKMGR_CACHE_DIR"

	// EnvOffline forbids network access.
	EnvOffline = "SKMGR_OFFLINE"

	// EnvLogLevel sets the log level.
	EnvLogLevel = "SKMGR_LOG_LEVEL"
)

// File is the on-disk shape of the config file.
//
// Every field is optional: an absent field means "no opinion", which is what
// lets an environment variable or a default take over rather than being
// overwritten by a zero value.
type File struct {
	// Home relocates skmgr's state directory.
	Home string `yaml:"home,omitempty"`

	// CacheDir relocates the download cache.
	CacheDir string `yaml:"cache_dir,omitempty"`

	// Offline forbids network access for every command.
	Offline *bool `yaml:"offline,omitempty"`

	// LogLevel is the default verbosity: debug, info, warn, or error.
	LogLevel string `yaml:"log_level,omitempty"`

	// Targets is the default agent list for `skmgr init` in a project where
	// nothing can be auto-detected.
	Targets []string `yaml:"targets,omitempty"`
}

// Config is the resolved settings a command should act on.
type Config struct {
	// Path is the config file that was read, empty if there was none.
	Path string

	// Home is skmgr's state directory.
	Home string

	// CacheDir is the download cache.
	CacheDir string

	// Offline forbids network access.
	Offline bool

	// LogLevel is the configured verbosity, empty if unset.
	LogLevel string

	// Targets is the configured default agent list, nil if unset.
	Targets []string
}

var (
	cacheMu     sync.Mutex
	cachedFile  *File
	cachedPath  string
	cachedError error
	fileLoaded  bool
)

// Current returns the resolved configuration.
//
// Callers deep in the tree need paths without threading a config value through
// every signature. Only the parsed file is cached: reading it on every path
// lookup would be wasteful, but the environment is cheap to re-read and must
// stay live, since a process that changes HOME expects the change to take.
//
// A load failure yields defaults so path lookups keep working; the error is
// available from Err, and the CLI reports it before running any command.
func Current() *Config {
	cfg, err := resolve()
	if err != nil {
		return defaults()
	}
	return cfg
}

// Err reports a failure to load the config file, if there was one.
func Err() error {
	_, err := resolve()
	return err
}

// Reset clears the cached config file. Tests use it after changing the
// environment; nothing else should need it.
func Reset() {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	cachedFile = nil
	cachedPath = ""
	cachedError = nil
	fileLoaded = false
}

// resolve combines the cached file with the current environment.
func resolve() (*Config, error) {
	file, path, err := loadFileOnce()
	if err != nil {
		return nil, err
	}

	return combine(file, path), nil
}

// loadFileOnce reads and caches the config file.
func loadFileOnce() (*File, string, error) {
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if fileLoaded {
		return cachedFile, cachedPath, cachedError
	}

	path := DefaultPath()
	file, err := readFile(path)

	fileLoaded = true
	cachedError = err

	switch {
	case err != nil:
		cachedFile, cachedPath = &File{}, ""
	case file == nil:
		cachedFile, cachedPath = &File{}, ""
	default:
		cachedFile, cachedPath = file, path
	}

	return cachedFile, cachedPath, cachedError
}

// defaults is the configuration with no file and no environment.
func defaults() *Config {
	home := defaultHome()
	return &Config{
		Home:     home,
		CacheDir: filepath.Join(home, "cache"),
	}
}

// DefaultPath returns the config file location: $SKMGR_CONFIG if set, and
// otherwise ~/.config/skmgr/config.yml.
//
// The XDG location is used rather than ~/.skmgr because that directory holds
// generated state, and a hand-edited file does not belong somewhere a user
// might reasonably delete to clear a cache.
func DefaultPath() string {
	if explicit := strings.TrimSpace(os.Getenv(EnvConfig)); explicit != "" {
		return explicit
	}

	if dir, err := os.UserConfigDir(); err == nil {
		return filepath.Join(dir, "skmgr", "config.yml")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".config", "skmgr", "config.yml")
}

// Load resolves settings from the config file, the environment, and defaults.
//
// A missing config file is not an error: most people never write one. A
// malformed one is, since silently ignoring it would leave the user staring at
// settings that never took effect.
func Load() (*Config, error) {
	path := DefaultPath()

	file, err := readFile(path)
	if err != nil {
		return nil, err
	}

	if file == nil {
		file, path = &File{}, ""
	}

	return combine(file, path), nil
}

// combine layers the environment over the config file over the defaults.
func combine(file *File, path string) *Config {
	cfg := &Config{Path: path}

	cfg.Home = expand(firstNonEmpty(
		os.Getenv(EnvHome),
		file.Home,
		defaultHome(),
	))

	cfg.CacheDir = expand(firstNonEmpty(
		os.Getenv(EnvCacheDir),
		file.CacheDir,
		filepath.Join(cfg.Home, "cache"),
	))

	cfg.LogLevel = firstNonEmpty(os.Getenv(EnvLogLevel), file.LogLevel)

	switch {
	case os.Getenv(EnvOffline) != "":
		cfg.Offline = truthy(os.Getenv(EnvOffline))
	case file.Offline != nil:
		cfg.Offline = *file.Offline
	}

	cfg.Targets = file.Targets

	return cfg
}

// readFile loads the config file, returning nil when there is none.
func readFile(path string) (*File, error) {
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var file File
	decoder := yaml.NewDecoder(strings.NewReader(string(data)))
	decoder.KnownFields(true)

	if err := decoder.Decode(&file); err != nil {
		// An empty file decodes to EOF, which is a valid "no opinion".
		if err.Error() == "EOF" {
			return &File{}, nil
		}
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	return &file, nil
}

// expand turns a configured directory into an absolute path, resolving a
// leading ~ first.
//
// Both steps matter. `~/skmgr-cache` is what a person writes in a config file
// and what a shell would have expanded for them, and a relative path would
// otherwise resolve against whatever directory the command happened to run in,
// scattering caches across the filesystem. Keeping paths absolute also lets Go
// apply the \\?\ prefix on Windows, which is what lifts the 260-character
// limit.
func expand(path string) string {
	if path == "" {
		return ""
	}

	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			rest := strings.TrimLeft(path[1:], `/\`)
			path = filepath.Join(home, rest)
		}
	}

	absolute, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return absolute
}

// defaultHome is ~/.skmgr, where generated state lives.
func defaultHome() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".skmgr"
	}
	return filepath.Join(home, ".skmgr")
}

// firstNonEmpty returns the first value that is set, after trimming.
func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// truthy reads an environment variable as a boolean, treating anything other
// than the explicit negatives as true. A variable that is set at all is
// normally meant to turn something on.
func truthy(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "0", "false", "no", "off":
		return false
	default:
		return true
	}
}
