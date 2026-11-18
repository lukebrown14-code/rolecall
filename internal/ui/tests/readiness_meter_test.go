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

// The detail lists each Document, the Resume and the Cover letter, with its two steps
// marked ✓ done or ○ still to do, and a meter with the count. The JD is an input and is
// not on the DOCS line.
func TestDetailListsEachDocumentReadyOrMissing(t *testing.T) {
	root := fixtureCopy(t, nil)
	view := chipped(runKeysSized(t, root, 120, 36, "3", "/docs-written", "enter"))
	for _, want := range []string{"✓ Resume ○ PDF   ✓ Cover letter ○ PDF", "■■□□ 2/4 ready"} {
		if !strings.Contains(view, want) {
			t.Errorf("detail lacks %q:\n%s", want, view)
		}
	}
	if strings.Contains(lineAfter(view, "DOCS"), "JD") {
		t.Errorf("the DOCS line counts the JD:\n%s", view)
	}
}

// ASCII mode draws the meter's cells as the ready and missing marks, + and _, which mean
// the same, and keeps the done/to-do marks apart.
func TestMeterFoldsToASCII(t *testing.T) {
	t.Setenv("ROLECALL_ASCII", "1")
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, "3", "/docs-written", "enter"))
	if !strings.Contains(view, "++__ 2/4 ready") || !strings.Contains(view, "+ Resume _ PDF") {
		t.Errorf("no ASCII meter:\n%s", view)
	}
}

// The TUI words a Document's state as done or still to do, never present or empty.
func TestDocumentWordingIsWrittenThenBuilt(t *testing.T) {
	banned := regexp.MustCompile(`\b(present|empty)\b`)
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"3", "/docs-written", "enter"}, {"3", "/docs-ready", "enter"}, {"2", "enter"}, {"?"}} {
		view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 36, keys...))
		for _, line := range strings.Split(view, "\n") {
			low := strings.ToLower(line)
			if !strings.Contains(low, "pdf") && !strings.Contains(low, "resume") && !strings.Contains(low, " jd") && !strings.Contains(low, "cover letter") {
				continue
			}
			if m := banned.FindString(low); m != "" {
				t.Errorf("keys %q: Document state worded %q: %q", keys, m, line)
			}
		}
	}
}
