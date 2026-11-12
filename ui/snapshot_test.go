package ui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// snapshotStates are the screens and pop-ups a design review looks at, named by what
// was pressed.
var snapshotStates = []struct {
	name string
	keys []string
	env  map[string]string // set for this state only, e.g. the light theme
}{
	{"01-leads", []string{"1"}, nil},
	{"02-drafts", []string{"2"}, nil},
	{"03-applied-all", []string{"3"}, nil},
	{"04-applied-interviewing", []string{"3", "tab", "tab"}, nil},
	{"05-detail", []string{"3", "enter"}, nil},
	{"06-detail-docs-tab", []string{"3", "enter", "right", "right", "right"}, nil},
	{"07-move-popup", []string{"1", "m"}, nil},
	{"08-add-form", []string{"a"}, nil},
	{"09-add-form-error", []string{"a", "enter"}, nil},
	{"10-help", []string{"?"}, nil},
	{"11-search-typing", []string{"/", "p", "l"}, nil},
	{"12-search-no-match", []string{"/", "z", "z", "q", "enter"}, nil},
	{"13-error", []string{"G", "e"}, nil},
	{"14-track-cycled", []string{"1", "t"}, nil},
	{"15-stats", []string{"4"}, nil},
	{"16-resume-reader", []string{"3", "/docs-written", "enter", "enter", "right", "right"}, nil},
	{"17-profile", []string{"5"}, nil},
	{"18-settings-search", []string{"6"}, nil},
	{"19-settings-documents", []string{"6", "j"}, nil},
	{"20-settings-tracking", []string{"6", "j", "j"}, nil},
	{"21-settings-machine", []string{"6", "j", "j", "j"}, nil},
	{"22-settings-search-popup", []string{"6", "tab", "enter"}, nil},
	{"23-settings-gate-popup", []string{"6", "j", "tab", "enter"}, nil},
	{"24-settings-track-add", []string{"6", "j", "j", "tab", "a"}, nil},
	{"25-settings-track-remove-refused", []string{"6", "j", "j", "tab", "x"}, nil},
	{"26-settings-track-rename", []string{"6", "j", "j", "tab", "r"}, nil},
	{"27-settings-machine-edit", []string{"6", "j", "j", "j", "tab", "enter"}, nil},
	{"28-light-applied-all", []string{"3"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"29-light-move-popup", []string{"1", "m"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"30-light-detail", []string{"3", "enter"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"31-light-profile", []string{"5"}, map[string]string{"ROLECALL_THEME": "light"}},
	{"32-ascii-applied-all", []string{"3"}, map[string]string{"ROLECALL_ASCII": "1"}},
	{"33-no-color-applied-all", []string{"3"}, map[string]string{"NO_COLOR": "1"}},
}

// TestSnapshots writes every snapshotState as .txt (plain) and .ansi (with colour) to
// $ROLECALL_SNAP_DIR, at $ROLECALL_SNAP_SIZE (default 120x36). Run it with
// scripts/snap.sh; it skips in a normal test run.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("ROLECALL_SNAP_DIR")
	if dir == "" {
		t.Skip("set ROLECALL_SNAP_DIR, or run scripts/snap.sh")
	}
	w, h := 120, 36
	if size := os.Getenv("ROLECALL_SNAP_SIZE"); size != "" {
		if _, err := fmt.Sscanf(size, "%dx%d", &w, &h); err != nil {
			t.Fatalf("ROLECALL_SNAP_SIZE %q: want WIDTHxHEIGHT", size)
		}
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, s := range snapshotStates {
		t.Run(s.name, func(t *testing.T) {
			for k, v := range s.env {
				t.Setenv(k, v)
			}
			view := runKeysSized(t, fixtureCopy(t, nil), w, h, s.keys...)
			base := filepath.Join(dir, fmt.Sprintf("%s-%dx%d", s.name, w, h))
			for ext, body := range map[string]string{".txt": ansi.Strip(view), ".ansi": view} {
				if err := os.WriteFile(base+ext, []byte(body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
