package diag_test

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/diag"
)

// start opens a log in a temp dir for one test and closes it after.
func start(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "state", "rolecall", "diagnostics.jsonl")
	if err := diag.Start(path); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(diag.Stop)
	return path
}

func entries(t *testing.T, path string) []map[string]any {
	t.Helper()
	diag.Stop()
	lines, err := diag.Tail(path, 1000)
	if err != nil {
		t.Fatal(err)
	}
	var out []map[string]any
	for _, line := range lines {
		var e map[string]any
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("not JSON: %q", line)
		}
		out = append(out, e)
	}
	return out
}

func TestPathFollowsXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	if got, want := diag.Path("/home/o"), "/home/o/.local/state/rolecall/diagnostics.jsonl"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
	t.Setenv("XDG_STATE_HOME", "relative") // not absolute: ignored, as the spec asks
	if got := diag.Path("/home/o"); !strings.HasPrefix(got, "/home/o/.local/state") {
		t.Errorf("relative XDG_STATE_HOME used: %q", got)
	}
	t.Setenv("XDG_STATE_HOME", "/xdg")
	if got, want := diag.Path("/home/o"), "/xdg/rolecall/diagnostics.jsonl"; got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestEnabledIsOnForDevBuildsUnlessTheEnvSaysOtherwise(t *testing.T) {
	for _, c := range []struct {
		env, version string
		want         bool
	}{
		{"", "dev", true}, {"", "1.0.0", false}, {"1", "1.0.0", true}, {"0", "dev", false}, {"yes", "1.0.0", false},
	} {
		t.Setenv(diag.EnvVar, c.env)
		if got := diag.Enabled(c.version); got != c.want {
			t.Errorf("Enabled(%q) with %s=%q = %v, want %v", c.version, diag.EnvVar, c.env, got, c.want)
		}
	}
}

func TestRecordingIsANoOpUntilStart(t *testing.T) {
	if diag.On() {
		t.Fatal("log open before Start")
	}
	diag.Record("ignored")
	diag.Error("ignored", errors.New("x"))
}

func TestStartCreatesAPrivateLogAndRecordsJSONLines(t *testing.T) {
	path := start(t)
	diag.Record("screen", "name", "Leads")
	diag.Error("feedback_error", errors.New("meta.yml line 4: bad"), "type", "*store.ParseError")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("log mode = %o, want 600", mode)
	}
	got := entries(t, path)
	if len(got) != 2 || got[0]["msg"] != "screen" || got[0]["name"] != "Leads" {
		t.Fatalf("entries = %v", got)
	}
	if got[1]["level"] != "ERROR" || got[1]["err"] != "meta.yml line 4: bad" {
		t.Errorf("error entry = %v", got[1])
	}
}

func TestStartRotatesALargeLog(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostics.jsonl")
	if err := os.WriteFile(path, make([]byte, 1<<20), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := diag.Start(path); err != nil {
		t.Fatal(err)
	}
	diag.Record("fresh")
	diag.Stop()
	if info, err := os.Stat(diag.RotatedPath(path)); err != nil || info.Size() != 1<<20 {
		t.Fatalf("rotated log missing or wrong size: %v %v", info, err)
	}
	if info, _ := os.Stat(path); info.Size() > 1000 {
		t.Errorf("new log not fresh: %d bytes", info.Size())
	}
}

func TestASessionStopsWritingAtItsCap(t *testing.T) {
	path := start(t)
	big := strings.Repeat("x", 64<<10)
	for range 100 { // 6.4 MiB asked for, past the 4 MiB cap
		diag.Record("big", "pad", big)
	}
	diag.Stop()
	info, _ := os.Stat(path)
	if info.Size() > 5<<20 {
		t.Errorf("log grew to %d bytes", info.Size())
	}
	lines, _ := diag.Tail(path, 1)
	if !strings.Contains(lines[0], "session limit reached") {
		t.Errorf("last line = %.80q, want the limit notice", lines[0])
	}
}

func TestGuardRecordsThePanicWithItsStackAndPanicsOn(t *testing.T) {
	path := start(t)
	func() {
		defer func() {
			if r := recover(); r != "boom" {
				t.Errorf("recovered %v, want the original panic", r)
			}
		}()
		defer diag.Guard("update")
		panicHere()
	}()
	got := entries(t, path)
	if len(got) != 1 || got[0]["msg"] != "panic" || got[0]["where"] != "update" || got[0]["value"] != "boom" {
		t.Fatalf("entries = %v", got)
	}
	if stack, _ := got[0]["stack"].(string); !strings.Contains(stack, "panicHere") {
		t.Errorf("stack does not reach the panic site:\n%s", stack)
	}
}

func panicHere() { panic("boom") }

func TestCommandNamesTheProgramNotItsArguments(t *testing.T) {
	path := start(t)
	ok := exec.Command("true", "secret-arg")
	ok.Path = "/usr/bin/true"
	diag.Command(ok, 12*time.Millisecond, nil, "")
	failed := exec.Command("/bin/sh", "-c", "exec rolecall-test-no-such-harness 'Write a CV for Acme'")
	err := failed.Run()
	diag.Command(failed, time.Second, err, "noise\nreal error")
	got := entries(t, path)
	if got[0]["msg"] != "command" || got[0]["program"] != "true" || got[0]["ms"] != 12.0 {
		t.Errorf("success entry = %v", got[0])
	}
	if got[1]["msg"] != "command_failed" || got[1]["program"] != "sh:rolecall-test-no-such-harness" || got[1]["exit"] != 127.0 || got[1]["stderr"] != "noise\nreal error" {
		t.Errorf("failure entry = %v", got[1])
	}
	for _, e := range got {
		b, _ := json.Marshal(e)
		if strings.Contains(string(b), "secret-arg") || strings.Contains(string(b), "Acme") {
			t.Errorf("entry carries arguments: %s", b)
		}
	}
}

func TestLastKeepsTheEndOfLongOutput(t *testing.T) {
	if got := diag.Last("  short \n", 10); got != "short" {
		t.Errorf("Last = %q", got)
	}
	if got := diag.Last("abcdefghij", 3); got != "…hij" {
		t.Errorf("Last = %q", got)
	}
}
