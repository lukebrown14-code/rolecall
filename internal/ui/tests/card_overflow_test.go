package ui_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// lastFilled is the last line of lines with text on it.
func lastFilled(lines []string) string {
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

// A card taller than its pane ends in the overflow cue, as the list beside it does, never
// a silent cut; the cue says where the rest is. NEXT is never what gets cut.
func TestShortCardEndsInTheOverflowCue(t *testing.T) {
	for _, tc := range []struct {
		name string
		keys []string
		cue  string
	}{
		{"preview", []string{"3"}, "more · enter open"},
		{"detail", []string{"3", "enter"}, "more · taller window shows all"},
	} {
		card := previewCard(t, nil, 120, 18, tc.keys...)
		text := strings.Join(card, "\n")
		if last := lastFilled(card); !regexp.MustCompile(`^↓ \d+ ` + regexp.QuoteMeta(tc.cue) + `$`).MatchString(last) {
			t.Errorf("%s: the cut card ends %q, want the overflow cue:\n%s", tc.name, last, text)
		}
		if !hasLineWith(text, "NEXT", "wait to hear back") {
			t.Errorf("%s: NEXT was cut:\n%s", tc.name, text)
		}
	}
}

// Short of room a card folds the rows the Sweep and the ad recorded (TYPE, PAY…) before
// it cuts anything, so NEXT and the TIMELINE keep their lines.
func TestShortCardFoldsTheAdFactsFirst(t *testing.T) {
	tall := strings.Join(previewCard(t, nil, 120, 44, "1", "/Data Engineer", "enter"), "\n")
	if !strings.Contains(tall, "PAY") || !strings.Contains(tall, "TYPE") {
		t.Fatalf("the card has no ad facts to fold:\n%s", tall)
	}
	text := strings.Join(previewCard(t, nil, 120, 24, "1", "/Data Engineer", "enter"), "\n")
	for _, want := range []string{"NEXT", "TIMELINE", "NOTES"} {
		if !strings.Contains(text, want) {
			t.Errorf("%s was cut before the ad facts folded:\n%s", want, text)
		}
	}
	if strings.Contains(text, "PAY") || strings.Contains(text, "TYPE") {
		t.Errorf("the ad facts did not fold:\n%s", text)
	}
}

// The detail header says the applied date only while the Status is Applied or later:
// moved back to Draft it was not sent, and undoing the move brings the date back.
func TestDetailHeaderAppliedOnlyOnceSent(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36)
	d.Type("3")
	d.Type("/Orbit")
	d.Type("enter")
	d.Type("enter")
	header := func() string { return lineWith(d.plain(), "‹ esc") }
	if !strings.Contains(header(), "applied 25 Sep") {
		t.Fatalf("Applied header lacks the applied date: %q", header())
	}
	d.Type("m")
	d.Type("d")
	if !strings.Contains(d.plain(), "✎ Draft") || strings.Contains(header(), "applied") {
		t.Errorf("Draft header still says applied: %q", header())
	}
	d.Type("u")
	if !strings.Contains(header(), "applied 25 Sep") {
		t.Errorf("undo did not restore the applied date: %q", header())
	}
}

// A not-needed Cover letter is — on both the card and the Docs tab, as the legend says.
func TestNotNeededIsOneGlyphEverywhere(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	m = driveModel(t, m, "3", "/docs-resume-only", "enter", "enter")
	if v := chipped(m.View().Content); !strings.Contains(v, "— Cover letter") {
		t.Errorf("card: no — on the Cover letter:\n%s", v)
	}
	m = driveModel(t, m, "left")
	v := chipped(m.View().Content)
	if !hasLineWith(v, "— Cover letter", "not needed · Resume only") || hasLineWith(v, "○ Cover letter") {
		t.Errorf("Docs tab: the not-needed Cover letter is not —:\n%s", v)
	}
	big := driveStart(t, fixtureCopy(t, nil), fixtureClock, 160, 60)
	help := helpText(driveModel(t, big, "?").View().Content)
	if !strings.Contains(help, "— not needed") {
		t.Errorf("legend does not say — not needed:\n%s", help)
	}
}

// Compact rows at 120 columns keep every date's short form whole: the title gives way
// first.
func TestCompactRowsKeepTheDateWhole(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 36, ui.WithCompactRows(true))
	d.Type("3")
	v := d.plain()
	date := regexp.MustCompile(`(applied|interviewing|offer|rejected|withdrawn) (~?\d+[a-z]+ ago|~?[A-Z][a-z]{2} \d{4}|today)`)
	rows := 0
	for _, l := range strings.Split(v, "\n") {
		row, _, _ := strings.Cut(l, "│")
		if !regexp.MustCompile(`^ .  [●◆★✕↩] `).MatchString(row) || strings.Contains(row, "──") {
			continue
		}
		rows++
		if !date.MatchString(row) {
			t.Errorf("compact row cuts its date: %q", row)
		}
	}
	if rows < 8 {
		t.Fatalf("found %d compact rows:\n%s", rows, v)
	}
	// The columns hold still: every row's date starts in one cell.
	col := -1
	for _, l := range strings.Split(v, "\n") {
		row, _, _ := strings.Cut(l, "│")
		if loc := date.FindStringIndex(row); loc != nil && !strings.Contains(row, "──") {
			at := ansi.StringWidth(row[:loc[0]])
			if col < 0 {
				col = at
			} else if at != col {
				t.Errorf("date column starts at %d, want %d: %q", at, col, row)
			}
		}
	}
}

// A Lead whose fit.md has text but whose meta.yml has no Fit says so on both: the card's
// "fit not assessed" and a FIT block labelled reasoning only, never a bare "Good." under it.
func TestCardFitStatesAgree(t *testing.T) {
	card := previewCard(t, nil, 120, 44, "1", "/Data Engineer", "enter")
	text := strings.Join(card, "\n")
	if !strings.Contains(text, "fit not assessed") {
		t.Fatalf("no unassessed Fit on the card:\n%s", text)
	}
	if slices.Contains(card, "FIT") || !slices.Contains(card, "FIT · reasoning only, not assessed") {
		t.Errorf("the FIT block reads as a second Fit:\n%s", text)
	}
}
