package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// cardOf is the wide frame's right-hand pane, the preview or the detail card: what follows
// the divider on each body line, between the rules that meet it.
func cardOf(view string) []string {
	var out []string
	in := false
	for _, l := range strings.Split(view, "\n") {
		switch {
		case strings.Contains(l, "┬"):
			in = true
			continue
		case strings.Contains(l, "┴"):
			in = false
		}
		if i := strings.LastIndex(l, "│"); in && i >= 0 {
			out = append(out, strings.TrimSpace(l[i+len("│"):]))
		}
	}
	return out
}

// sheetOf is the bottom sheet: the lines between the rule over it and the frame's bottom rule.
func sheetOf(view string) []string {
	lines := strings.Split(view, "\n")
	var rules []int
	for i, l := range lines {
		if t := strings.TrimSpace(l); t != "" && strings.Trim(t, "─") == "" {
			rules = append(rules, i)
		}
	}
	if len(rules) < 2 { //nolint:mnd // the sheet's rule and the bottom rule
		return nil
	}
	return lines[rules[len(rules)-2]+1 : rules[len(rules)-1]]
}

func previewCard(t *testing.T, edit func(string), w, h int, keys ...string) []string {
	t.Helper()
	m := driveStart(t, fixtureCopy(t, edit), fixtureClock, w, h, ui.WithHarness("h {skill} {dir}"))
	return cardOf(chipped(driveModel(t, m, keys...).View().Content))
}

// fileEdit is a Workspace edit that writes the last of parts to the path the rest name.
func fileEdit(parts ...string) func(string) {
	text := parts[len(parts)-1]
	return func(root string) {
		p := filepath.Join(append([]string{root}, parts[:len(parts)-1]...)...)
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			panic(err)
		}
	}
}

// fieldValue is a card row "LABEL   value": the label, then two or more spaces, then a value.
var fieldValue = regexp.MustCompile(`^([A-Z][A-Z-]*(?: [A-Z][A-Z-]*)*)( {2,})\S`)

// Every labelled value in a card starts in one column, SPONSORSHIP's and APPLICANTS' too,
// each with at least two spaces after its label.
func TestCardValuesStartInOneColumn(t *testing.T) {
	for _, name := range []string{"Quokka", "Fern"} {
		card := previewCard(t, nil, 120, 40, "1", "/"+name, "enter")
		col, labels := -1, 0
		for _, l := range card {
			m := fieldValue.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			labels++
			if c := len(m[1]) + len(m[2]); col < 0 {
				col = c
			} else if c != col {
				t.Errorf("%s: %q starts its value at %d, not %d:\n%s", name, m[1], c, col, strings.Join(card, "\n"))
			}
		}
		if labels < 4 { //nolint:mnd // NEXT, DOCS, NOTES and a fact at least
			t.Fatalf("%s: found %d labelled rows:\n%s", name, labels, strings.Join(card, "\n"))
		}
	}
}

// The card's subtitle is Company · Track: the close date, posting age and badges each show
// once, on their own labelled row, and a key the next step offers is not offered again.
func TestCardSaysEachFactOnce(t *testing.T) {
	closes := func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "fern", "meta.yml")
		b, _ := os.ReadFile(meta)
		_ = os.WriteFile(meta, []byte(strings.Replace(string(b), "track: Platform\n", "track: Platform\ncloses: 2026-10-14\nfit: Good\n", 1)), 0o600)
		_ = os.WriteFile(filepath.Join(root, "jobs", "SWE", "fern", "jd.md"), []byte("# JD\n\nBuild platforms.\n"), 0o600)
	}
	card := previewCard(t, closes, 120, 40, "1", "/Fern", "enter")
	text := strings.Join(card, "\n")
	if card[1] != "Fern Systems · Platform" {
		t.Errorf("subtitle = %q, want Company · Track:\n%s", card[1], text)
	}
	for _, once := range []string{"closes in", "27d ago", "p prepare docs"} {
		if n := strings.Count(text, once); n != 1 {
			t.Errorf("%q shows %d times:\n%s", once, n, text)
		}
	}
	if !strings.Contains(lineWith(text, "NEXT"), "p prepare docs") {
		t.Errorf("the next step lost its key:\n%s", text)
	}
}

// With no close date the card has no CLOSES row and no dash for one.
func TestCardOmitsCloseDateItDoesNotHave(t *testing.T) {
	text := strings.Join(previewCard(t, nil, 120, 40, "1", "/Fern", "enter"), "\n")
	if strings.Contains(text, "CLOSES") || strings.Contains(text, "—") {
		t.Errorf("card shows a close date it does not have:\n%s", text)
	}
}

// The age the list rows no longer show is on the card's Status line.
func TestCardStatusLineSaysTheAge(t *testing.T) {
	for _, tc := range []struct{ screen, name, want string }{
		{"1", "Fern", "◌ Lead · added 7d ago"},
		{"3", "Orbit", "● Applied · 6d in Applied"},
	} {
		text := strings.Join(previewCard(t, nil, 120, 40, tc.screen, "/"+tc.name, "enter"), "\n")
		if !hasLineWith(text, tc.want, "fit not assessed") {
			t.Errorf("%s: Status line lacks %q:\n%s", tc.name, tc.want, text)
		}
	}
}

// The preview's spare room reads the Fit's reasoning, then the JD, cut with a cue that says
// how to read the rest; the verdict line, which the Status line already says, is left out.
func TestPreviewShowsTheFitThenTheJD(t *testing.T) {
	var b strings.Builder
	b.WriteString("# Fit — Fern Systems, Platform Engineer\n\n**Verdict: Good**\n\nSummary: a platform role close to the Owner's last two.\nMatches: Go, Kubernetes, CI\n\n")
	for range 12 {
		b.WriteString("The case for it runs on years of platform work and on-call.\n\n")
	}
	fit := fileEdit("jobs", "SWE", "fern", "fit.md", b.String())
	jd := fileEdit("jobs", "SWE", "fern", "jd.md", "# Platform Engineer\n\nRun the build farm.\n")
	card := previewCard(t, func(root string) { fit(root); jd(root) }, 120, 44, "1", "/Fern", "enter")
	text := strings.Join(card, "\n")
	at := slices.Index(card, "FIT")
	if at < 0 {
		t.Fatalf("no FIT block:\n%s", text)
	}
	if !strings.HasPrefix(card[at+1], "Summary: a platform role") || strings.Contains(text, "Verdict") {
		t.Errorf("FIT does not start at the reasoning:\n%s", text)
	}
	if !strings.Contains(text, "more · enter open") {
		t.Errorf("a cut FIT block has no cue:\n%s", text)
	}
	if slices.Index(card, "JD") < 0 || !strings.Contains(text, "Run the build farm.") {
		t.Errorf("no JD under the FIT block:\n%s", text)
	}
}

// The detail card's link is one inline row with the key that opens it.
func TestCardLinkIsOneRow(t *testing.T) {
	text := strings.Join(previewCard(t, nil, 120, 40, "3", "/docs-written", "enter", "enter"), "\n")
	if !hasLineWith(text, "LINK", "https://example.test/shared", "w open") {
		t.Errorf("LINK is not one row:\n%s", text)
	}
}

// The detail opens on the tab last chosen, and J / K step to the next and previous row
// without leaving it.
func TestDetailRemembersTabAndStepsRows(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	v := plain(driveModel(t, m, "1", "enter", "right", "esc", "j", "enter"))
	if !strings.Contains(v, "[Fit]") || !strings.Contains(v, "row 2 of") {
		t.Fatalf("the second detail did not open on Fit:\n%s", v)
	}
	m = driveModel(t, m, "1", "enter", "right", "J")
	if v := plain(m); !strings.Contains(v, "[Fit]") || !strings.Contains(v, "row 2 of") || !strings.Contains(v, "· detail") {
		t.Fatalf("J did not show the next row's detail on Fit:\n%s", v)
	}
	if v := plain(driveModel(t, m, "K")); !strings.Contains(v, "row 1 of") || !strings.Contains(v, "· detail") {
		t.Fatalf("K did not go back a row:\n%s", v)
	}
	if v := plain(driveModel(t, m, "K", "K")); !strings.Contains(v, "row 1 of") {
		t.Fatalf("K past the first row moved:\n%s", v)
	}
	if v := plain(driveModel(t, m, "esc")); !strings.Contains(v, "row 2 of") || strings.Contains(v, "· detail") {
		t.Fatalf("esc after J did not land on the row J showed:\n%s", v)
	}
	help := plain(driveModel(t, m, "?"))
	if !strings.Contains(help, "next row") || !strings.Contains(help, "previous row") {
		t.Errorf("? help does not name J and K:\n%s", help)
	}
	if v := plain(driveModel(t, m, ":", "next row", "enter")); !strings.Contains(v, "row 3 of") {
		t.Errorf("the palette's next row did not step:\n%s", v)
	}
}

// The medium bottom sheet keeps the next step in view on every tab, and says the Status
// at most twice: the title chip and the timeline.
func TestSheetKeepsNextInView(t *testing.T) {
	for _, size := range [][2]int{{72, 24}, {99, 30}} {
		m := driveStart(t, fixtureCopy(t, nil), fixtureClock, size[0], size[1])
		m = driveModel(t, m, "3", "/Orbit", "enter", "enter")
		for tab := range 5 {
			sheet := strings.Join(sheetOf(plain(m)), "\n")
			if !strings.Contains(sheet, "NEXT") {
				t.Errorf("%dx%d tab %d: no NEXT in the sheet:\n%s", size[0], size[1], tab, sheet)
			}
			if tab == 0 && strings.Count(sheet, "● Applied") > 2 { //nolint:mnd // the chip and the timeline
				t.Errorf("%dx%d: the sheet repeats the Status:\n%s", size[0], size[1], sheet)
			}
			m = driveModel(t, m, "right")
		}
	}
}

// The narrow detail's strip says the Status, documents and Fit, so its Overview does not
// repeat them, and the fixture Lead's timeline and notes fit without a cue.
func TestNarrowOverviewDoesNotRepeatTheStrip(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 64, 24)
	v := plain(driveModel(t, m, "1", "/Fern", "enter", "enter"))
	if strings.Contains(v, "DOCS") || strings.Count(v, "fit ") != 1 || strings.Contains(v, "◌ Lead ·") {
		t.Errorf("the Overview repeats the strip:\n%s", v)
	}
	if !strings.Contains(v, "TIMELINE") || !strings.Contains(v, "NOTES") || strings.Contains(v, "↓ ") {
		t.Errorf("the Overview does not fit:\n%s", v)
	}
}

// The Docs tab lists the Documents the DOCS meter counts, under that name, with the PDF
// row standing while out/ is empty; Gaps, which the meter leaves out, says so.
func TestDocsTabListsTheMetersDocuments(t *testing.T) {
	m := driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40)
	v := plain(driveModel(t, m, "3", "/docs-written", "enter", "enter", "left"))
	if !strings.Contains(v, "[Docs]") {
		t.Fatalf("not on the Docs tab:\n%s", v)
	}
	for _, l := range strings.Split(v, "\n") {
		if left, _, _ := strings.Cut(l, "│"); strings.TrimSpace(left) == "SOURCE" {
			t.Errorf("Docs tab keeps the Source heading:\n%s", v)
		}
	}
	if !strings.Contains(v, "DOCUMENTS") {
		t.Errorf("Docs tab heading is not Documents:\n%s", v)
	}
	if !hasLineWith(v, "PDF", "not built yet") || !hasLineWith(v, "Gaps", "not counted") {
		t.Errorf("Docs tab does not list the meter's Documents:\n%s", v)
	}
	built := plain(driveModel(t, driveStart(t, fixtureCopy(t, nil), fixtureClock, 120, 40), "3", "/docs-ready", "enter", "enter", "left"))
	if strings.Contains(built, "not built yet") {
		t.Errorf("a built Application says its PDF is not built:\n%s", built)
	}
}
