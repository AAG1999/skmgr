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

// Package log provides skmgr's leveled output.
//
// Diagnostics go to stderr so that machine-readable output on stdout (such as
// `list --json`) stays clean and pipeable.
package log

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// Level controls which messages are emitted.
type Level int

const (
	// LevelDebug shows everything, including per-step tracing.
	LevelDebug Level = iota
	// LevelInfo is the default: progress and results, no tracing.
	LevelInfo
	// LevelWarn hides progress but keeps warnings.
	LevelWarn
	// LevelError shows only errors.
	LevelError
)

// String renders the level name used in message prefixes.
func (l Level) String() string {
	switch l {
	case LevelDebug:
		return "debug"
	case LevelInfo:
		return "info"
	case LevelWarn:
		return "warn"
	case LevelError:
		return "error"
	default:
		return "unknown"
	}
}

// ParseLevel maps a user-supplied level name onto a Level.
func ParseLevel(name string) (Level, error) {
	switch name {
	case "debug":
		return LevelDebug, nil
	case "info", "":
		return LevelInfo, nil
	case "warn", "warning":
		return LevelWarn, nil
	case "error":
		return LevelError, nil
	default:
		return LevelInfo, fmt.Errorf("unknown log level %q (valid: debug, info, warn, error)", name)
	}
}

// Logger writes leveled diagnostics to a writer.
type Logger struct {
	mu    sync.Mutex
	level Level
	out   io.Writer
}

// New returns a Logger writing to out at the given level.
func New(out io.Writer, level Level) *Logger {
	return &Logger{level: level, out: out}
}

// SetLevel changes the threshold.
func (l *Logger) SetLevel(level Level) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

// Level reports the current threshold.
func (l *Logger) Level() Level {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.level
}

// SetOutput redirects the logger, mainly so tests can capture output.
func (l *Logger) SetOutput(out io.Writer) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.out = out
}

// Enabled reports whether a message at this level would be written.
func (l *Logger) Enabled(level Level) bool {
	return level >= l.Level()
}

func (l *Logger) logf(level Level, prefix string, format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if level < l.level || l.out == nil {
		return
	}

	msg := fmt.Sprintf(format, args...)

	// A failed write to the log sink has nowhere to be reported: the reporting
	// channel is the thing that just failed. Dropping the message is the only
	// option, and it must not disturb the operation being logged.
	_, _ = fmt.Fprintf(l.out, "%s%s\n", prefix, msg)
}

// Debugf writes tracing detail, shown only with --verbose.
func (l *Logger) Debugf(format string, args ...any) {
	l.logf(LevelDebug, "debug: ", format, args...)
}

// Infof writes normal progress output.
func (l *Logger) Infof(format string, args ...any) {
	l.logf(LevelInfo, "", format, args...)
}

// Warnf writes a recoverable problem the user should see.
func (l *Logger) Warnf(format string, args ...any) {
	l.logf(LevelWarn, "warning: ", format, args...)
}

// Errorf writes an error. Commands normally return errors instead of calling
// this; it exists for failures discovered where an error cannot be returned.
func (l *Logger) Errorf(format string, args ...any) {
	l.logf(LevelError, "error: ", format, args...)
}

// std is the process-wide logger used by commands and internal packages.
var std = New(os.Stderr, LevelInfo)

// Default returns the process-wide logger.
func Default() *Logger { return std }

// SetLevel sets the level on the process-wide logger.
func SetLevel(level Level) { std.SetLevel(level) }

// SetOutput redirects the process-wide logger.
func SetOutput(out io.Writer) { std.SetOutput(out) }

// Debugf logs to the process-wide logger.
func Debugf(format string, args ...any) { std.Debugf(format, args...) }

// Infof logs to the process-wide logger.
func Infof(format string, args ...any) { std.Infof(format, args...) }

// Warnf logs to the process-wide logger.
func Warnf(format string, args ...any) { std.Warnf(format, args...) }

// Errorf logs to the process-wide logger.
func Errorf(format string, args ...any) { std.Errorf(format, args...) }
