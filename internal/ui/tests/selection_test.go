package ui_test

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
)

const selectionWidth = 100

// finalView runs the whole program on the fixture Workspace, sends keys (one Tab per "\t"), and returns the final screen.
func finalView(t *testing.T, keys ...string) string {
	t.Helper()
	return finalViewAt(t, fixtureNow, keys...)
}

func finalViewAt(t *testing.T, now time.Time, keys ...string) string {
	t.Helper()
	m := driveStart(t, fixtureCopy(t, nil), func() time.Time { return now }, selectionWidth, 40)
	return driveModel(t, m, keys...).View().Content
}

// selectedLine is the cursor row's first line: the one carrying the selection bar ▌.
func selectedLine(t *testing.T, view string) string {
	t.Helper()
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(chipped(line), "▌ ") {
			return line
		}
	}
	t.Fatalf("no selected row in:\n%s", view)
	return ""
}

// selectionBg is the selection bar's background as it appears in an SGR sequence.
const selectionBg = "48;2;47;85;96" // #2F5560

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
	plain := chipped(line)
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
				t.Fatalf("selection bar is %d cells wide, want %d: %q", got, want, chipped(line))
			}
		})
	}
}

// The flag glyph can be ambiguous-width in some terminals and fonts; it must never be the last cell of the bar.
func TestFlaggedSelectedRowKeepsPaddingAfterFlag(t *testing.T) {
	bar := barText(selectedLine(t, finalViewAt(t, time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), "3")))
	if !strings.Contains(bar, "⚑ 3mo") || !strings.HasSuffix(bar, " ") {
		t.Fatalf("flagged row is not padded after its last label: %q", bar)
	}
}

func TestHelpBarOffersOnlyKeysValidForTheRow(t *testing.T) {
	const all = "e edit,m move,u undo,x dismiss,n notes,d download"
	t.Run("malformed row offers detail only", func(t *testing.T) {
		raw := finalView(t, "G") // the malformed row sorts last on Leads
		view := chipped(raw)
		if !strings.Contains(view, "⚠") || !strings.Contains(chipped(selectedLine(t, raw)), "⚠") {
			t.Fatalf("cursor is not on the malformed row:\n%s", view)
		}
		for _, k := range strings.Split(all, ",") {
			if strings.Contains(view, k) {
				t.Errorf("malformed row offers %q:\n%s", k, view)
			}
		}
		if !strings.Contains(view, "Enter open") {
			t.Errorf("malformed row hides detail:\n%s", view)
		}
	})
	t.Run("healthy row offers detail, edit, move and dismiss", func(t *testing.T) {
		// At this width n notes makes way for the pinned a add and / search; ? lists it.
		view := chipped(finalView(t))
		for _, k := range []string{"Enter open", "e edit", "m move", "x dismiss", "a add", "/ search"} {
			if !strings.Contains(view, k) {
				t.Errorf("healthy row hides %q:\n%s", k, view)
			}
		}
	})
	t.Run("undo appears only once there is a move to undo", func(t *testing.T) {
		if view := chipped(finalView(t)); strings.Contains(view, "u undo") {
			t.Errorf("empty undo stack offers undo:\n%s", view)
		}
		if view := chipped(finalView(t, "x")); !strings.Contains(view, "u undo") {
			t.Errorf("undo hidden after a move:\n%s", view)
		}
	})
	t.Run("detail view does not offer itself", func(t *testing.T) {
		if view := chipped(finalView(t, "v")); strings.Contains(view, "enter open") {
			t.Errorf("detail view offers detail:\n%s", view)
		}
	})
}
