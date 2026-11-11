package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/ui"
)

// fakeHarness stands in for the interactive Harness boundary: it records the shell
// command the model asked for, optionally writes what the agent would, and reports
// an exit status.
type fakeHarness struct {
	calls [][]string
	write func()
	err   error
}

func (f *fakeHarness) runner(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
	return func() tea.Msg {
		f.calls = append(f.calls, append([]string(nil), c.Args...))
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

func plain(m tea.Model) string { return ansi.Strip(m.View().Content) }

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
	want := `claude "/assess-fit '` + folder + `'"`
	if len(f.calls) != 1 || f.calls[0][0] != "/bin/sh" || f.calls[0][1] != "-c" || f.calls[0][2] != want {
		t.Fatalf("harness command = %v, want /bin/sh -c %q", f.calls, want)
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
	if !strings.Contains(view, "Platform Engineer") || !strings.Contains(view, "Fit: Good") {
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
			if !strings.Contains(view, "f assess fit") {
				t.Fatalf("help lacks f:\n%s", view)
			}
			if got := strings.Contains(view, "p prepare docs"); got != tc.wantP {
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
	if view := plain(m); !strings.Contains(view, "Set harness in ~/.config/rolecall/config.toml") {
		t.Fatalf("missing hint:\n%s", view)
	}
	if len(f.calls) != 0 {
		t.Fatalf("harness ran with no harness configured: %v", f.calls)
	}
	m = driveModel(t, m, "esc", "?")
	if view := plain(m); strings.Contains(view, "assess fit") || strings.Contains(view, "prepare docs") {
		t.Fatalf("f/p still listed without a harness:\n%s", view)
	}
}

func TestHandoffPrepareDocumentGateDoesNotRun(t *testing.T) {
	root := fixtureCopy(t, nil)
	f := &fakeHarness{}
	m := driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(f.runner), ui.WithHarness("x {skill} {dir}"))
	m = driveModel(t, m, "/Engineer #1", "enter", "3", "p")
	if len(f.calls) != 0 {
		t.Fatalf("p ran on an Interviewing Application: %v", f.calls)
	}
}
