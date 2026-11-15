package diag

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Command records one finished external program: which program, how long it ran and how
// it ended, plus the end of stderr when it failed. It names the program, never its
// arguments, which can carry a Harness prompt or a job's text; for `sh -c SCRIPT` it
// names the script's first word instead of the shell.
func Command(c *exec.Cmd, ran time.Duration, err error, stderr string) {
	if !On() {
		return
	}
	attrs := []any{"program", Program(c), "ms", ran.Milliseconds()}
	if err == nil {
		Record("command", attrs...)
		return
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		attrs = append(attrs, "exit", exit.ExitCode())
	}
	if stderr != "" {
		attrs = append(attrs, "stderr", Last(stderr, TailBytes))
	}
	Error("command_failed", err, attrs...)
}

// Program is the name an entry gives c: the executable's base name, or for a shell
// running a script, the script's first word (after an `exec`).
func Program(c *exec.Cmd) string {
	name := filepath.Base(c.Path)
	if len(c.Args) < 3 || c.Args[1] != "-c" || (name != "sh" && name != "bash") {
		return name
	}
	words := strings.Fields(c.Args[2])
	if len(words) > 1 && words[0] == "exec" {
		words = words[1:]
	}
	if len(words) == 0 {
		return name
	}
	return name + ":" + filepath.Base(strings.Trim(words[0], `"'`))
}
