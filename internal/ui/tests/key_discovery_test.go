package ui_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// helpPages is every page of the ? pop-up at a size after keys, as the Owner pages it.
func helpPages(t *testing.T, w, h int, keys ...string) string {
	t.Helper()
	m := driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, w, h), append(keys, "?")...)
	var pages []string
	for range 20 {
		v := chipped(m.View().Content)
		if len(pages) > 0 && pages[len(pages)-1] == v {
			break
		}
		pages = append(pages, v)
		m = driveModel(t, m, "down")
	}
	return strings.Join(pages, "\n")
}

// Below the wide size class the help is a screen of its own: no tab bar or pop-up border,
// the page from the first line, and one footer with its title and keys. Wide, it stays a
// pop-up over the frame. ROLECALL_PLAIN draws it full-screen at any width, with no rules.
func TestHelpIsFullScreenBelowWide(t *testing.T) {
	for _, tc := range []struct {
		w, h  int
		popUp bool
	}{{166, 44, true}, {120, 36, true}, {90, 30, false}, {72, 24, false}, {56, 14, false}} {
		lines := strings.Split(chipped(runKeysSized(t, fixtureCopy(t, nil), tc.w, tc.h, "1", "?")), "\n")
		view := strings.Join(lines, "\n")
		if got := strings.Contains(view, "╭─ Keys · Leads"); got != tc.popUp {
			t.Errorf("%dx%d: pop-up = %v, want %v:\n%s", tc.w, tc.h, got, tc.popUp, view)
		}
		if tc.popUp {
			continue
		}
		footer := strings.TrimSpace(lines[len(lines)-1])
		if len(lines) != tc.h || !strings.HasPrefix(lines[0], "  NAVIGATE") || strings.Contains(view, "Drafts") ||
			!strings.HasPrefix(footer, "Keys · Leads · page 1 of ") || !strings.HasSuffix(footer, "↑↓ page  esc close") {
			t.Errorf("%dx%d: the help is not a screen of its own:\n%s", tc.w, tc.h, view)
		}
	}
	t.Setenv("ROLECALL_PLAIN", "1")
	view := ansi.Strip(runKeysSized(t, fixtureCopy(t, nil), 160, 44, "1", "?"))
	if strings.ContainsAny(view, "─│╭╰") || !strings.Contains(view, "Keys · Leads") || !strings.Contains(view, "NAVIGATE") {
		t.Errorf("plain help draws rules or lost its words:\n%s", view)
	}
}

// Paging the full-screen help with ↓ shows the next page and esc closes it back to the
// screen it was opened from.
func TestFullScreenHelpPagesAndCloses(t *testing.T) {
	m := driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 72, 24), "1", "?")
	m.View() // the help counts its pages as it is drawn, as the program draws after each key
	m = driveModel(t, m, "down")
	if v := chipped(m.View().Content); !strings.Contains(v, "page 2 of ") || !strings.Contains(v, "GLYPHS") {
		t.Errorf("↓ did not turn to the second page:\n%s", v)
	}
	if v := chipped(driveModel(t, m, "esc").View().Content); strings.Contains(v, "Keys · Leads") || !strings.Contains(v, "Drafts") {
		t.Errorf("esc did not close the help:\n%s", v)
	}
}

// The help defines the words a list row shows, in the rows' own wording, and the two
// dismissed views.
func TestHelpDefinesTheRowWords(t *testing.T) {
	help := helpPages(t, 120, 36, "1")
	for _, want := range []string{"ROW WORDS", "posted 12d", "past the age limit", "ad summary only",
		"contract fits visa cap", "12-mo contract > 6-mo visa cap", "sponsors visa", "no sponsorship",
		"⚑ 6w", "Strong Good Marginal Poor", "Dismissed", "Filtered", "dismissed by rules; restorable"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	for _, gone := range []string{"blurb only", "needs sponsorship", "-month cap"} {
		if strings.Contains(help, gone) {
			t.Errorf("help keeps the old badge wording %q", gone)
		}
	}
}

// The help's AI skills say what each Plugin skill does, the key that hands a row to it,
// and how to connect a Harness while none is set.
func TestHelpListsTheAISkills(t *testing.T) {
	help := helpPages(t, 120, 36, "1")
	for _, want := range []string{"AI SKILLS", "connect a Harness first: h or Settings › This machine",
		"f  assess-fit", "p  prepare-documents", "F  draft-follow-up", "capture-job", "find-jobs", "create-profile", "help"} {
		if !strings.Contains(help, want) {
			t.Errorf("help lacks %q:\n%s", want, help)
		}
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 60, ui.WithHarness("x {skill} {dir}"))
	if v := chipped(driveModel(t, m, "1", "?").View().Content); strings.Contains(v, "connect a Harness first") ||
		!strings.Contains(v, "f p F hand the Application to your Harness") {
		t.Errorf("with a Harness the help still asks for one:\n%s", v)
	}
}

// With Applications in the Workspace and no Harness, h is still offered in the help and the
// palette, and running it from the palette lands on Settings › This machine › Harness.
func TestConnectHarnessOutlivesTheWelcome(t *testing.T) {
	if help := helpText(runKeysSized(t, fixtureCopy(t, nil), 120, 40, "3", "?")); !strings.Contains(help, "h connect a Harness") {
		t.Fatalf("help lacks h connect a Harness: %q", help)
	}
	v := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3", ":", "connect", "enter"))
	if !strings.Contains(v, "Settings › This machine") || !strings.Contains(selectedRow(t, v), "Harness") {
		t.Fatalf("palette h did not open the Harness row:\n%s", v)
	}
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40, ui.WithHarness("x {skill} {dir}"))
	if help := helpText(driveModel(t, m, "3", "?").View().Content); strings.Contains(help, "connect a Harness") {
		t.Fatalf("help offers h with a Harness set: %q", help)
	}
}
