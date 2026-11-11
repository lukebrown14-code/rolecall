package ui_test

import (
	"bytes"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/lukebrown14-code/rolecall/store"
	"github.com/lukebrown14-code/rolecall/ui"
)

const selectionWidth = 100

// finalView runs the whole program on the fixture Workspace, sends keys (one Tab per "\t"), and returns the final screen.
func finalView(t *testing.T, keys ...string) string {
	t.Helper()
	return finalViewAt(t, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), keys...)
}

func finalViewAt(t *testing.T, now time.Time, keys ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	m, err := ui.New(s, func() time.Time { return now })
	if err != nil {
		t.Fatal(err)
	}
	tm := teatest.NewTestModel(t, m, teatest.WithInitialTermSize(selectionWidth, 40))
	teatest.WaitFor(t, tm.Output(), func(b []byte) bool { return bytes.Contains(b, []byte("Leads 4")) })
	for _, k := range keys {
		if k == "\t" {
			tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
		} else {
			tm.Type(k)
		}
	}
	tm.Type("q")
	return tm.FinalModel(t, teatest.WithFinalTimeout(3*time.Second)).View().Content
}

// selectedLine is the cursor row's first line: the one carrying the selection bar ▌.
func selectedLine(t *testing.T, view string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(ansi.Strip(line), "▌ ") {
			return line
		}
	}
	t.Fatalf("no selected row in:\n%s", view)
	return ""
}

// selectionBg is the selection bar's background as it appears in an SGR sequence.
const selectionBg = "48;2;31;61;61"

// barText is the text drawn on the selection background in a styled line.
func barText(line string) string {
	var b strings.Builder
	for _, m := range regexp.MustCompile(`\x1b\[([0-9;]*)m([^\x1b]*)`).FindAllStringSubmatch(line, -1) {
		if strings.Contains(m[1], selectionBg) {
			b.WriteString(m[2])
		}
	}
	return b.String()
}

// listColumn is the width of the list column: everything left of the preview's divider.
func listColumn(line string) int {
	plain := ansi.Strip(line)
	if i := strings.Index(plain, "│"); i >= 0 {
		return ansi.StringWidth(plain[:i])
	}
	return ansi.StringWidth(plain)
}

// The selection bar spans the list column, less a one-cell margin each side, in every view.
func TestSelectionBarSpansTheListColumnInEveryView(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []string
	}{
		{"Leads", []string{"1"}},
		{"Dismissed", []string{"1", "\t"}},
		{"Draft", []string{"2"}},
		{"All", []string{"3"}},
		{"Applied", []string{"3", "\t"}},
		{"Interviewing", []string{"3", "\t", "\t"}},
		{"Offer", []string{"3", "\t", "\t", "\t"}},
		{"Closed", []string{"3", "\t", "\t", "\t", "\t"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			line := selectedLine(t, finalView(t, tc.keys...))
			if got, want := ansi.StringWidth(barText(line)), listColumn(line)-2; got != want {
				t.Fatalf("selection bar is %d cells wide, want %d: %q", got, want, ansi.Strip(line))
			}
		})
	}
}

// The ⚑ glyph is ambiguous-width in some terminals and fonts; it must never be the last cell of the bar.
func TestFlaggedSelectedRowKeepsPaddingAfterFlag(t *testing.T) {
	bar := barText(selectedLine(t, finalViewAt(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "3")))
	if !strings.Contains(bar, "⚑ quiet") || !strings.HasSuffix(bar, " ") {
		t.Fatalf("flagged row is not padded after its last label: %q", bar)
	}
}

func TestHelpBarOffersOnlyKeysValidForTheRow(t *testing.T) {
	const all = "e edit,m move,u undo,x dismiss,n notes,d download"
	t.Run("malformed row offers detail only", func(t *testing.T) {
		raw := finalView(t, "G") // the malformed row sorts last on Leads
		view := ansi.Strip(raw)
		if !strings.Contains(view, "⚠") || !strings.Contains(ansi.Strip(selectedLine(t, raw)), "⚠") {
			t.Fatalf("cursor is not on the malformed row:\n%s", view)
		}
		for _, k := range strings.Split(all, ",") {
			if strings.Contains(view, k) {
				t.Errorf("malformed row offers %q:\n%s", k, view)
			}
		}
		if !strings.Contains(view, "enter open") {
			t.Errorf("malformed row hides detail:\n%s", view)
		}
	})
	t.Run("healthy row offers detail, edit, move and notes", func(t *testing.T) {
		view := ansi.Strip(finalView(t))
		for _, k := range []string{"enter open", "e edit", "m move", "x dismiss", "n notes"} {
			if !strings.Contains(view, k) {
				t.Errorf("healthy row hides %q:\n%s", k, view)
			}
		}
	})
	t.Run("undo appears only once there is a move to undo", func(t *testing.T) {
		if view := ansi.Strip(finalView(t)); strings.Contains(view, "u undo") {
			t.Errorf("empty undo stack offers undo:\n%s", view)
		}
		if view := ansi.Strip(finalView(t, "x")); !strings.Contains(view, "u undo") {
			t.Errorf("undo hidden after a move:\n%s", view)
		}
	})
	t.Run("detail view does not offer itself", func(t *testing.T) {
		if view := ansi.Strip(finalView(t, "v")); strings.Contains(view, "enter open") {
			t.Errorf("detail view offers detail:\n%s", view)
		}
	})
}
