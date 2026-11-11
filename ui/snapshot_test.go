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
}{
	{"01-leads", []string{"1"}},
	{"02-drafts", []string{"2"}},
	{"03-applied-all", []string{"3"}},
	{"04-applied-interviewing", []string{"3", "tab", "tab"}},
	{"05-detail", []string{"3", "enter"}},
	{"06-detail-docs-tab", []string{"3", "enter", "right", "right", "right"}},
	{"07-move-popup", []string{"1", "m"}},
	{"08-add-form", []string{"a"}},
	{"09-add-form-error", []string{"a", "enter"}},
	{"10-help", []string{"?"}},
	{"11-search-typing", []string{"/", "p", "l"}},
	{"12-search-no-match", []string{"/", "z", "z", "q", "enter"}},
	{"13-error", []string{"G", "e"}},
	{"14-track-cycled", []string{"1", "t"}},
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
		view := runKeysSized(t, fixtureCopy(t, nil), w, h, s.keys...)
		base := filepath.Join(dir, fmt.Sprintf("%s-%dx%d", s.name, w, h))
		for ext, body := range map[string]string{".txt": ansi.Strip(view), ".ansi": view} {
			if err := os.WriteFile(base+ext, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
}
