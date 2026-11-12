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

// The header's track hint reads "t track", not a bare "t".
func TestTrackHintReadsTrack(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "1"))
	top, _, _ := strings.Cut(view, "\n")
	if !strings.Contains(top, "t track") {
		t.Fatalf("header track hint is not labelled: %q", top)
	}
}

// The cursor keys sit in the ? pop-up's Navigate column, off the key line.
func TestHelpNavigateColumnHasCursorKeys(t *testing.T) {
	view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, "?"))
	if !strings.Contains(view, "j k ↑↓") || !strings.Contains(view, "g G") {
		t.Fatalf("Navigate column lacks the cursor keys:\n%s", view)
	}
}
