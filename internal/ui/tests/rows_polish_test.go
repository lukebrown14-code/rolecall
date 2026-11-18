package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// fernCloses gives the fixture's Fern Lead a close date.
func fernCloses(date string) func(string) {
	return func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "fern", "meta.yml")
		b, _ := os.ReadFile(meta)
		_ = os.WriteFile(meta, bytes.Replace(b, []byte("track: Platform\n"), []byte("track: Platform\ncloses: "+date+"\n"), 1), 0o600)
	}
}

// Close dates read by urgency in the row, the preview and the detail title: amber words
// within a week, ▲ within two days, "expired 3 Sep" once past.
func TestCloseDateUrgency(t *testing.T) {
	for _, tc := range []struct{ closes, want string }{
		{"2026-10-07", "closes in 6d (soon)"},
		{"2026-10-03", "closes in 2d ▲"},
		{"2026-10-01", "closes today ▲"},
		{"2026-09-03", "expired 3 Sep"},
		{"2027-03-01", "closes Mar 2027"},
	} {
		t.Run(tc.closes, func(t *testing.T) {
			root := fixtureCopy(t, fernCloses(tc.closes))
			list := chipped(runKeys(t, root, 120, "1", "/Fern", "enter"))
			if strings.Count(list, tc.want) < 2 { // the row and the preview
				t.Fatalf("want %q in the row and the preview:\n%s", tc.want, list)
			}
			if detail := chipped(runKeys(t, root, 120, "1", "/Fern", "enter", "enter")); !strings.Contains(detail, tc.want) {
				t.Fatalf("detail lacks %q:\n%s", tc.want, detail)
			}
		})
	}
}

// Rows show a Fit as its word alone, a missing close date as nothing at all, and old
// dates as the month rather than "21mo ago".
func TestRowWording(t *testing.T) {
	leads := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1"))
	if strings.Contains(leads, "no close date") || strings.Contains(leads, "Platform · —") || !strings.Contains(leads, "Fern Systems · Platform · posted 27d") {
		t.Fatalf("Leads rows:\n%s", leads)
	}
	for _, l := range strings.Split(leads, "\n") {
		if row, _, _ := strings.Cut(l, "│"); regexp.MustCompile(`\bfit\b`).MatchString(row) || strings.HasSuffix(strings.TrimRight(row, " "), "—") {
			t.Fatalf("a Lead row shows the word fit or a bare dash: %q", row)
		}
	}
	applied := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, want := range []string{"⚑ 9mo  Good", "applied ~Jan 2025"} {
		if !strings.Contains(applied, want) {
			t.Fatalf("Applied rows lack %q:\n%s", want, applied)
		}
	}
	if strings.Contains(applied, "mo ago") {
		t.Fatalf("an old date still reads as months ago:\n%s", applied)
	}
}

// The preview and the detail view say what to do next, with a key that works there.
func TestNextActionLine(t *testing.T) {
	for _, tc := range []struct {
		name      string
		keys      []string
		want, key string
	}{
		{"lead without a JD", []string{"1"}, "NEXT         add the JD", "enter open"},
		{"applied", []string{"3"}, "NEXT         wait to hear back", "m move"},
		{"follow-up due", []string{"3", "/Example", "enter"}, "FOLLOW-UP    due · no change in 9mo", "l followed up"},
		{"offer", []string{"3", "tab", "tab", "tab"}, "NEXT         decide on the offer", "m move"},
		{"detail on the JD tab", []string{"1", "enter"}, "NEXT         add the JD", "o open jd.md"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := chipped(runKeys(t, fixtureCopy(t, nil), 120, tc.keys...))
			// the narrow card puts the key on the line under the words
			if row := lineWith(v, tc.want); !strings.Contains(row, tc.key) && !strings.Contains(lineAfter(v, tc.want), tc.key) {
				t.Fatalf("lacks %q … %q:\n%s", tc.want, tc.key, v)
			}
		})
	}
}

// Moving to Offer celebrates once, in the toast; a rejection stays quiet.
func TestOfferCelebration(t *testing.T) {
	root := fixtureCopy(t, nil)
	// The move lands on the Application's new home: the Offer view, with its row selected,
	// and the celebration as one toast (not a body banner over an empty list).
	v := chipped(runKeys(t, root, 120, "3", "tab", "m", "o"))
	if !strings.Contains(v, "★ OFFER · Orbit Labs ★  6 days after applying, 7 sent") {
		t.Fatalf("no celebration:\n%s", v)
	}
	if !strings.Contains(v, "Pipeline › Offer") {
		t.Fatalf("did not land on the Offer view:\n%s", v)
	}
	if strings.Contains(v, "to follow it") {
		t.Fatalf("redundant follow hint still shown:\n%s", v)
	}
	if v := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3", "tab", "m", "r")); strings.Contains(v, "OFFER") {
		t.Fatalf("a rejection celebrated:\n%s", v)
	}
}

// A Workspace without Applications teaches the first three moves.
func TestFirstRunScreen(t *testing.T) {
	root := fixtureCopy(t, func(root string) { _ = os.RemoveAll(filepath.Join(root, "jobs")) })
	v := chipped(runKeys(t, root, 80, "1"))
	for _, want := range []string{"▞ rolecall · no Applications yet", "a  add your first Lead", "?  see every key"} {
		if !strings.Contains(v, want) {
			t.Fatalf("first run lacks %q:\n%s", want, v)
		}
	}
}

// o switches Leads between deadline and newest added, the Leads header names the order
// for as long as it holds, and the help offers the other order.
func TestLeadsSortToggle(t *testing.T) {
	root := fixtureCopy(t, fernCloses("2026-09-01"))
	byClose := chipped(runKeys(t, root, 140, "1"))
	if !strings.Contains(byClose, "Leads · 6 · by deadline") || !strings.Contains(byClose, "Expired · 1") ||
		!strings.Contains(helpText(runKeys(t, root, 140, "1", "?")), "o sort newest") {
		t.Fatalf("default order:\n%s", byClose)
	}
	newest := chipped(runKeys(t, root, 120, "1", "o"))
	if strings.Contains(newest, "Expired") || !strings.Contains(newest, "Leads newest first") {
		t.Fatalf("newest first:\n%s", newest)
	}
	if v := chipped(runKeys(t, root, 120, "1", "o", "j")); !strings.Contains(v, "Leads · 7 · newest first") || strings.Contains(v, "by deadline") {
		t.Fatalf("the header does not keep the order once the toast has gone:\n%s", v)
	}
	if v := helpText(runKeys(t, root, 140, "1", "o", "j", "?")); !strings.Contains(v, "o sort by deadline") {
		t.Fatalf("help does not offer the deadline order:\n%s", v)
	}
	if v := chipped(runKeys(t, root, 120, "1", "o", "o", "j")); !strings.Contains(v, "Leads · 6 · by deadline") || strings.Contains(v, "newest first") {
		t.Fatalf("o again does not go back to the deadline order:\n%s", v)
	}
}

// The wordmark's mark is never a Status glyph, in either mode.
func TestWordmarkIsNotAStatusGlyph(t *testing.T) {
	if v := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1")); !strings.HasPrefix(v, " ▞ rolecall") {
		t.Fatalf("wordmark: %q", strings.SplitN(v, "\n", 2)[0])
	}
	t.Setenv("ROLECALL_ASCII", "1")
	if v := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1")); !strings.HasPrefix(v, " R rolecall") {
		t.Fatalf("ASCII wordmark: %q", strings.SplitN(v, "\n", 2)[0])
	}
}
