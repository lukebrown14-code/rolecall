package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// fakeHarness stands in for the interactive Harness boundary: it records the shell
// command the model asked for, optionally writes what the agent would, and reports
// an exit status.
type fakeHarness struct {
	calls [][]string
	dirs  []string
	write func()
	err   error
}

func (f *fakeHarness) runner(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
	return func() tea.Msg {
		f.calls = append(f.calls, append([]string(nil), c.Args...))
		f.dirs = append(f.dirs, c.Dir)
		if f.write != nil {
			f.write()
		}
		return done(f.err)
	}
}

// exitError is what a real /bin/sh returns when it exits non-zero.
func exitError(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("/bin/sh", "-c", "exit "+strconv.Itoa(code)).Run()
	if err == nil {
		t.Fatal("expected a non-zero exit")
	}
	return err
}

// setFit rewrites meta.yml to carry a Fit, as the assess-fit skill does.
func setFit(t *testing.T, folder, fit string) {
	t.Helper()
	path := filepath.Join(folder, "meta.yml")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var out strings.Builder
	inserted := false
	for _, line := range strings.SplitAfter(string(b), "\n") {
		if !inserted && strings.HasPrefix(line, "status_log:") {
			out.WriteString("fit: " + fit + "\n")
			inserted = true
		}
		out.WriteString(line)
	}
	if err := os.WriteFile(path, []byte(out.String()), 0600); err != nil {
		t.Fatal(err)
	}
}

func plain(m tea.Model) string { return chipped(m.View().Content) }

func TestHandoffAssessFitWritesFitAndRescans(t *testing.T) {
	root := fixtureCopy(t, nil)
	f := &fakeHarness{}
	folder := filepath.Join(root, "jobs", "SWE", "fern")
	f.write = func() {
		setFit(t, folder, "Strong")
		if err := os.WriteFile(filepath.Join(folder, "fit.md"), []byte("Strong: matches the Platform track"), 0600); err != nil {
			t.Fatal(err)
		}
	}
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness(`claude "/{skill} {dir}"`))
	m = driveModel(t, m, "/fern", "enter", "f")

	view := plain(m)
	if !strings.Contains(view, "Strong") {
		t.Fatalf("row does not show the new Fit:\n%s", view)
	}
	if !strings.Contains(view, "1 Leads") {
		t.Fatalf("screen changed across the Handoff:\n%s", view)
	}
	// The folder travels as a positional parameter, never inside the script text.
	want := []string{"/bin/sh", "-c", `claude "/${2} ${1}"`, "rolecall-harness", folder, "assess-fit"}
	if len(f.calls) != 1 || !slices.Equal(f.calls[0], want) {
		t.Fatalf("harness command = %q, want %q", f.calls, want)
	}
	if f.dirs[0] != root {
		t.Fatalf("Harness started in %q, want the Workspace %q", f.dirs[0], root)
	}
}

func TestHandoffFromDetailKeepsTabAndApp(t *testing.T) {
	root := fixtureCopy(t, nil)
	f := &fakeHarness{}
	folder := filepath.Join(root, "jobs", "SWE", "fern")
	f.write = func() { setFit(t, folder, "Good") }
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness("x {skill} {dir}"))
	m = driveModel(t, m, "/fern", "enter", "enter") // open the detail view
	m = driveModel(t, m, "right")                   // JD → Fit
	m = driveModel(t, m, "f")

	view := plain(m)
	// The detail view is still open on the same Application, on the Fit tab.
	if !strings.Contains(view, "Platform Engineer") || !strings.Contains(view, "fit Good") {
		t.Fatalf("detail view lost its place or did not rescan:\n%s", view)
	}
}

func TestHandoffExitStatusInFeedbackLine(t *testing.T) {
	root := fixtureCopy(t, nil)
	f := &fakeHarness{err: exitError(t, 7)}
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness("x {skill} {dir}"))
	m = driveModel(t, m, "/fern", "enter", "f")

	view := plain(m)
	if !strings.Contains(view, "Harness exited with status 7") {
		t.Fatalf("exit status not reported:\n%s", view)
	}
}

func TestHandoffAvailability(t *testing.T) {
	for _, tc := range []struct {
		name, filter, screen string
		wantP                bool
	}{
		{"lead has both", "/fern", "1", true},
		{"draft has both", "/Engineer #1", "2", true},
		{"interviewing has f only", "/Engineer #1", "3", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixtureCopy(t, nil)
			m := driveStart(t, root, fixtureClock, 200, 40, ui.WithHarness("x {skill} {dir}"))
			m = driveModel(t, m, tc.filter, "enter", tc.screen, "?")
			view := plain(m)
			if !helpLists(view, "f assess fit") {
				t.Fatalf("help lacks f:\n%s", view)
			}
			if got := helpLists(view, "p prepare docs"); got != tc.wantP {
				t.Fatalf("p shown = %v, want %v:\n%s", got, tc.wantP, view)
			}
		})
	}
}

func TestHandoffWithoutHarnessShowsHint(t *testing.T) {
	root := fixtureCopy(t, nil)
	f := &fakeHarness{}
	m := driveStart(t, root, fixtureClock, 200, 40, ui.WithExternalRunner(f.runner))
	m = driveModel(t, m, "/fern", "enter")
	m = driveModel(t, m, "f")
	if view := plain(m); !strings.Contains(view, "No Harness set · 6 Settings › This machine › Harness") {
		t.Fatalf("missing hint:\n%s", view)
	}
	if len(f.calls) != 0 {
		t.Fatalf("harness ran with no harness configured: %v", f.calls)
	}
	// The error offers the key that fixes it, and h lands on the Harness row.
	if view := plain(m); !strings.Contains(view, "h connect a Harness") {
		t.Fatalf("the no-Harness error offers no key:\n%s", view)
	}
	if view := plain(driveModel(t, m, "h")); !strings.Contains(view, "Settings › This machine") ||
		!strings.Contains(selectedRow(t, view), "Harness") || strings.Contains(view, "No Harness set") {
		t.Fatalf("h did not open Settings on the Harness row:\n%s", view)
	}
	// Help names f and p, dimmed and marked as needing a Harness, so they can be found.
	view := plain(driveModel(t, m, "esc", "?"))
	if !helpLists(view, "f assess fit") || !helpLists(view, "p prepare docs") || !strings.Contains(view, "(needs a Harness)") {
		t.Fatalf("help hides f/p without a harness:\n%s", view)
	}
}

func TestHandoffPrepareDocumentGateDoesNotRun(t *testing.T) {
	root := fixtureCopy(t, nil)
	f := &fakeHarness{}
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness("x {skill} {dir}"))
	driveModel(t, m, "/Engineer #1", "enter", "3", "p")
	if len(f.calls) != 0 {
		t.Fatalf("p ran on an Interviewing Application: %v", f.calls)
	}
}
