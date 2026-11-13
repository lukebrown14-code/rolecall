package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// A Status date after today is a data error, flagged rather than shown as "in 8mo".
func TestFutureStatusDatesAreFlagged(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "orbit", "meta.yml")
		b, _ := os.ReadFile(meta)
		edited := strings.Replace(string(b), "Applied, at: 2026-09-25", "Applied, at: 2027-09-25", 1)
		if err := os.WriteFile(meta, []byte(edited), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	view := ansi.Strip(runKeys(t, root, 120, "3", "enter", "enter"))
	if !strings.Contains(view, "⚠ future date") {
		t.Fatalf("future Applied date not flagged:\n%s", view)
	}
}

// e on a malformed row names the next step as well as the parse error.
func TestMalformedErrorShowsTheNextStep(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "G", "m", "d"))
	if !strings.Contains(view, "Can't read SWE/malformed · meta.yml line 4: indented lines must be status_log entries") || !strings.Contains(view, "e fix in editor") {
		t.Fatalf("malformed edit lacks its next step:\n%s", view)
	}
}

// The preview's file marks reflect content: an empty resume.md is not marked written.
func TestEmptyResumeIsNotMarkedWritten(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "2"))
	if strings.Contains(view, "✓ resume") {
		t.Fatalf("an empty resume.md was marked ✓:\n%s", view)
	}
}

// The key line under a keyed pop-up shows context, not a second copy of the border keys.
// Every pop-up owns the keyboard, so the key line lists the pop-up's keys, never the
// screen's beneath it.
func TestPopupKeyLineShowsThePopupKeys(t *testing.T) {
	for _, keys := range [][]string{{"1", "m"}, {"a"}, {"?"}} {
		view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, keys...))
		last := view[strings.LastIndex(view, "\n")+1:]
		if !strings.Contains(last, "esc") {
			t.Fatalf("%v: key line lacks the pop-up's esc: %q", keys, last)
		}
		if strings.Contains(last, "/ search") || strings.Contains(last, "1-6 screen") {
			t.Fatalf("%v: key line shows the screen's keys under a pop-up: %q", keys, last)
		}
	}
}

// The track filter sits on the context row, labelled once by its key: "t  [All]  SWE
// Platform", or "t [All]▸" where the row is narrow. The tab bar carries no filter.
func TestTrackFilterReadsItsKeyOnce(t *testing.T) {
	for _, size := range [][3]int{{120, 36, 0}, {80, 24, 0}, {60, 16, 1}} {
		lines := strings.Split(ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), size[0], size[1], "3")), "\n")
		ctx := lines[3-size[2]] // the short class drops the gap under the tab bar
		want := "t  [All 17]  SWE 10  Platform 1"
		if size[2] == 1 {
			want = "t [All 17]▸" // collapsed to one piece when narrow
		}
		if !strings.Contains(ctx, want) || strings.Count(ctx, " t ") > 1 {
			t.Errorf("%dx%d: context row %q, want it to end %q", size[0], size[1], ctx, want)
		}
		if strings.Contains(lines[0], "All") || strings.Contains(lines[0], "track") {
			t.Errorf("%dx%d: tab bar carries the filter: %q", size[0], size[1], lines[0])
		}
	}
}

// The cursor keys sit in the ? pop-up's Navigate column, off the key line.
func TestHelpNavigateColumnHasCursorKeys(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "?"))
	if !strings.Contains(view, "j k ↑↓") || !strings.Contains(view, "g G") {
		t.Fatalf("Navigate column lacks the cursor keys:\n%s", view)
	}
}
