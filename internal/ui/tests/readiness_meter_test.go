package ui_test

import (
	"regexp"
	"strings"
	"testing"
)

// meterRun is a run of meter cells: ■ ready, □ missing.
func meterRun(line string) string {
	var b strings.Builder
	for _, r := range line {
		if r == '■' || r == '□' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// List rows carry no meter, as the design draws them: the detail's DOCS line and a Draft's
// second line say how far the Documents are.
func TestRowsCarryNoMeter(t *testing.T) {
	for _, s := range columnSizes {
		for _, r := range rowLines(listView(t, s[0], s[1])) {
			if run := meterRun(r); run != "" {
				t.Errorf("%dx%d: row carries a meter %q: %q", s[0], s[1], run, r)
			}
		}
	}
}

// The detail lists each Document with ✓ ready or ○ missing, and a meter with the count.
func TestDetailListsEachDocumentReadyOrMissing(t *testing.T) {
	root := fixtureCopy(t, nil)
	view := chipped(runKeysSized(t, root, 120, 36, "3", "/docs-written", "enter"))
	for _, want := range []string{"○ JD", "✓ Resume", "✓ Cover letter", "○ PDF", "■■□□ 2/4 ready"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail lacks %q:\n%s", want, view)
		}
	}
}

// ASCII mode draws the meter as # and :, and keeps the ready/missing marks apart.
func TestMeterFoldsToASCII(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/docs-written", "enter"))
	if !strings.Contains(view, "##::") || !strings.Contains(view, "_ JD") || !strings.Contains(view, "+ Resume") {
		t.Errorf("no ASCII meter:\n%s", view)
	}
}

// The TUI says "ready" and "missing" for a Document, never present, written or empty.
func TestDocumentWordingIsReadyOrMissing(t *testing.T) {
	banned := regexp.MustCompile(`\b(present|written|empty)\b`)
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"3", "/docs-written", "enter"}, {"3", "/docs-ready", "enter"}, {"2", "enter"}, {"?"}} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, keys...))
		for _, line := range strings.Split(view, "\n") {
			// The fixture's docs-written folder names a row; the card can share its line.
			low := strings.ReplaceAll(strings.ToLower(line), "docs-written", "")
			if !strings.Contains(low, "pdf") && !strings.Contains(low, "resume") && !strings.Contains(low, " jd") && !strings.Contains(low, "cover letter") {
				continue
			}
			if m := banned.FindString(low); m != "" && !strings.Contains(low, "agent writing") {
				t.Errorf("keys %q: Document state worded %q: %q", keys, m, line)
			}
		}
	}
}
