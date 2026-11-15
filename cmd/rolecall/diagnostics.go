package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/diag"
)

const defaultDiagnosticsLines = 40

// startDiagnostics opens the diagnostics log when this build or ROLECALL_DIAGNOSTICS
// turns it on, records the session's start, and returns the call that records its end.
// A log that cannot be opened never stops rolecall; it says why on stderr and goes on.
func startDiagnostics(args []string) func(code int) {
	if !diag.Enabled(version) {
		return func(int) {}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return func(int) {}
	}
	if err := diag.Start(diag.Path(home)); err != nil {
		fmt.Fprintf(os.Stderr, "diagnostics log unavailable: %v\n", err)
		return func(int) {}
	}
	started := time.Now()
	diag.Record("session_start", "command", subcommand(args), "version", version, "os", runtime.GOOS, "arch", runtime.GOARCH,
		"go", runtime.Version(), "term", os.Getenv("TERM"), "colorterm", os.Getenv("COLORTERM"),
		"no_color", os.Getenv("NO_COLOR") != "", "ascii", os.Getenv("ROLECALL_ASCII") != "")
	return func(code int) {
		diag.Record("session_end", "exit", code, "ms", time.Since(started).Milliseconds())
		diag.Stop()
	}
}

// subcommand names the run for the log without its arguments, which hold paths.
func subcommand(args []string) string {
	if len(args) > 0 && (args[0] == "init" || args[0] == "doctor") {
		return args[0]
	}
	return "tui"
}

// diagnosticsCommand is `rolecall diagnostics`: where the log is, whether it is on, and
// its latest entries.
func diagnosticsCommand(args []string, out, errOut io.Writer) int {
	flags := flag.NewFlagSet("diagnostics", flag.ContinueOnError)
	flags.SetOutput(errOut)
	n := flags.Int("n", defaultDiagnosticsLines, "number of latest entries to show")
	raw := flags.Bool("json", false, "print entries as the JSON lines they are stored as")
	pathOnly := flags.Bool("path", false, "print only the log's path")
	flags.Usage = func() {
		_, _ = fmt.Fprintf(errOut, "usage: rolecall diagnostics [-n N] [--json] [--path]\n\n"+
			"Shows the local diagnostics log: panics, failed programs, feedback-line errors and\n"+
			"slow updates. It never leaves this machine. Development builds keep it; set\n"+
			"%s=1 to keep it in a release, or 0 to stop it.\n", diag.EnvVar)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); errors.Is(err, flag.ErrHelp) {
		return 0
	} else if err != nil {
		return usageErrorExitCode
	}
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	path := diag.Path(home)
	if *pathOnly {
		_, _ = fmt.Fprintln(out, path)
		return 0
	}
	state := "on"
	if !diag.Enabled(version) {
		state = "off (set " + diag.EnvVar + "=1 to turn it on)"
	}
	_, _ = fmt.Fprintf(out, "Diagnostics %s · %s\n", state, path)
	lines, err := diag.Tail(path, *n)
	if errors.Is(err, fs.ErrNotExist) {
		_, _ = fmt.Fprintln(out, "No entries yet.")
		return 0
	}
	if err != nil {
		_, _ = fmt.Fprintln(errOut, err)
		return 1
	}
	_, _ = fmt.Fprintln(out, "Review before sharing: entries name paths, programs and the errors rolecall showed.")
	for _, line := range lines {
		if *raw {
			_, _ = fmt.Fprintln(out, line)
		} else {
			_, _ = fmt.Fprintln(out, formatEntry(line))
		}
	}
	return 0
}

// formatEntry is one JSON entry as a line to read: time, event, then key=value pairs in
// name order. A stack goes on the lines after, indented. A line that is not JSON is shown
// as it is.
func formatEntry(line string) string {
	var e map[string]any
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		return line
	}
	at, _ := e["time"].(string)
	if t, err := time.Parse(time.RFC3339Nano, at); err == nil {
		at = t.Local().Format("2006-01-02 15:04:05")
	}
	msg, _ := e["msg"].(string)
	if level, _ := e["level"].(string); level == "ERROR" {
		msg = "ERROR " + msg
	}
	stack, _ := e["stack"].(string)
	for _, k := range []string{"time", "level", "msg", "stack"} {
		delete(e, k)
	}
	keys := make([]string, 0, len(e))
	for k := range e {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString(strings.TrimSpace(at + "  " + msg))
	for _, k := range keys {
		fmt.Fprintf(&b, " %s=%v", k, quoteIfSpaced(e[k]))
	}
	if stack != "" {
		b.WriteString("\n    " + strings.ReplaceAll(strings.TrimSpace(stack), "\n", "\n    "))
	}
	return b.String()
}

func quoteIfSpaced(v any) any {
	if s, ok := v.(string); ok && strings.ContainsAny(s, " \n\t") {
		return fmt.Sprintf("%q", s)
	}
	return v
}
