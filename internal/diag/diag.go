// Package diag is rolecall's local diagnostics log (ADR-0006): a JSON Lines file on the
// Owner's own machine recording panics, failed commands, feedback-line errors and slow
// updates, so a problem can be traced after the fact. Nothing is sent anywhere.
//
// Entries name programs, paths, Statuses and screens, never file contents or the
// arguments a program was given. The log is off until Start is called; every recording
// function is then a no-op, so tests and disabled builds pay nothing.
package diag

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"sync"
)

// EnvVar turns the log on ("1") or off ("0"), overriding the build's default.
const EnvVar = "ROLECALL_DIAGNOSTICS"

const (
	// rotateBytes is the size at which Start moves the log aside to start a fresh one.
	rotateBytes = 1 << 20
	// sessionBytes caps what one session writes, so a runaway loop cannot fill the disk.
	sessionBytes = 4 << 20
	// TailBytes is how much of a failed program's stderr an entry keeps: the end, where
	// the error usually is.
	TailBytes = 2000
	// maxLine is the longest entry Tail reads; a stack trace fits well inside it.
	maxLine  = 1 << 20
	dirMode  = 0o700
	fileMode = 0o600
)

var (
	mu     sync.Mutex
	logger *slog.Logger
	file   *os.File
)

// Path is the log file: rolecall/diagnostics.jsonl under $XDG_STATE_HOME when that is an
// absolute path, as the XDG spec asks, else under home/.local/state.
func Path(home string) string {
	if dir := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "rolecall", "diagnostics.jsonl")
	}
	return filepath.Join(home, ".local", "state", "rolecall", "diagnostics.jsonl")
}

// RotatedPath is where Start moves a full log: one previous generation is kept.
func RotatedPath(path string) string {
	return strings.TrimSuffix(path, ".jsonl") + ".1.jsonl"
}

// Enabled reports whether this run should log: ROLECALL_DIAGNOSTICS when it is "1" or
// "0", else on for development builds (version "dev") and off for releases.
func Enabled(version string) bool {
	switch os.Getenv(EnvVar) {
	case "1":
		return true
	case "0":
		return false
	}
	return version == "dev"
}

// Start opens the log at path for appending, first moving it to RotatedPath when it has
// grown past rotateBytes. Until Stop, the recording functions write to it.
func Start(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), dirMode); err != nil {
		return err
	}
	if info, err := os.Stat(path); err == nil && info.Size() >= rotateBytes {
		if err := os.Rename(path, RotatedPath(path)); err != nil {
			return err
		}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, fileMode)
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
	}
	file = f
	logger = slog.New(slog.NewJSONHandler(&capped{w: f, left: sessionBytes}, nil))
	return nil
}

// Stop closes the log; recording is a no-op again afterwards.
func Stop() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		_ = file.Close()
	}
	file, logger = nil, nil
}

// On reports whether the log is open, for callers that would otherwise build an entry's
// fields for nothing.
func On() bool {
	mu.Lock()
	defer mu.Unlock()
	return logger != nil
}

// Record writes one entry: event names what happened, attrs are slog key-value pairs.
func Record(event string, attrs ...any) {
	mu.Lock()
	l := logger
	mu.Unlock()
	if l != nil {
		l.Info(event, attrs...)
	}
}

// Error writes one entry at error level, with err's text under "err".
func Error(event string, err error, attrs ...any) {
	mu.Lock()
	l := logger
	mu.Unlock()
	if l != nil {
		l.Error(event, append([]any{"err", errText(err)}, attrs...)...)
	}
}

// Panic records a recovered panic with the stack it unwound from. where names the phase
// (update, view, command). The caller re-panics, so the program's own recovery still
// restores the terminal.
func Panic(where string, r any) {
	Record("panic", "where", where, "value", fmt.Sprint(r), "stack", string(debug.Stack()))
}

// Guard is for a deferred call: it records a panic in progress and panics on with it.
//
//	defer diag.Guard("update")
func Guard(where string) {
	if r := recover(); r != nil {
		Panic(where, r)
		panic(r)
	}
}

// Tail returns the last n lines of the log at path, oldest first.
func Tail(path string, n int) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close() //nolint:errcheck // read-only
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(nil, maxLine)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > n {
			lines = lines[1:]
		}
	}
	return lines, scanner.Err()
}

// Last keeps the end of s, at most limit bytes, for stderr worth recording.
func Last(s string, limit int) string {
	s = strings.TrimSpace(s)
	if len(s) <= limit {
		return s
	}
	return "…" + s[len(s)-limit:]
}

func errText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// errLogFull is written once when a session reaches sessionBytes.
var errLogFull = errors.New("diagnostics: session limit reached; later entries dropped")

// capped passes writes through until left runs out, then writes one notice and drops the
// rest of the session's entries.
type capped struct {
	w    io.Writer
	left int
	full bool
}

func (c *capped) Write(p []byte) (int, error) {
	if c.full {
		return len(p), nil
	}
	if len(p) > c.left {
		c.full = true
		_, err := fmt.Fprintf(c.w, "{\"msg\":%q}\n", errLogFull.Error())
		return len(p), err
	}
	c.left -= len(p)
	return c.w.Write(p)
}
