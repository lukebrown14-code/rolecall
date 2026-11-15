package ui_test

import (
	"bytes"
	"os"
	"path/filepath"
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
// within a week, ▲ within two days, "closed 3 Sep" once past.
func TestCloseDateUrgency(t *testing.T) {
	for _, tc := range []struct{ closes, want string }{
		{"2026-10-07", "closes in 6d"},
		{"2026-10-03", "closes in 2d ▲"},
		{"2026-10-01", "closes today ▲"},
		{"2026-09-03", "closed 3 Sep"},
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

// Rows label Fit, show a missing close date once as a dash, and give old dates as the
// month rather than "21mo ago".
func TestRowWording(t *testing.T) {
	leads := chipped(runKeys(t, fixtureCopy(t, nil), 120, "1"))
	if strings.Contains(leads, "no close date") || !strings.Contains(leads, "Fern Systems · Platform · —") || strings.Contains(leads, "fit —") {
		t.Fatalf("Leads rows:\n%s", leads)
	}
	applied := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3"))
	for _, want := range []string{" Good", "applied ~Jan 2025", "● 9mo"} {
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
		{"lead without a JD", []string{"1"}, "NEXT    add the JD", "enter open"},
		{"applied", []string{"3"}, "NEXT    wait to hear back", "m move"},
		{"follow-up due", []string{"3", "/Example", "enter"}, "FOLLOW-UP  due 259 days ago", "m move"},
		{"offer", []string{"3", "tab", "tab", "tab"}, "NEXT    decide on the offer", "m move"},
		{"detail on the JD tab", []string{"1", "enter"}, "NEXT    add the JD", "e open jd.md"},
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
	if !strings.Contains(v, "★ OFFER · Orbit Labs ★  7 applied · 6 days") {
		t.Fatalf("no celebration:\n%s", v)
	}
	if !strings.Contains(v, "Applied › Offer") {
		t.Fatalf("did not land on the Offer view:\n%s", v)
	}
	if strings.Contains(v, "press ← → to follow it") {
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

// o switches Leads between close date and newest added, and the key line offers the
// other order.
func TestLeadsSortToggle(t *testing.T) {
	root := fixtureCopy(t, fernCloses("2026-09-01"))
	byClose := chipped(runKeys(t, root, 140, "1"))
	if !strings.Contains(byClose, "Expired · 1") || !strings.Contains(byClose, "o sort newest") {
		t.Fatalf("default order:\n%s", byClose)
	}
	newest := chipped(runKeys(t, root, 120, "1", "o"))
	if strings.Contains(newest, "Expired") || !strings.Contains(newest, "Leads newest first") {
		t.Fatalf("newest first:\n%s", newest)
	}
	if v := chipped(runKeys(t, root, 140, "1", "o", "j")); !strings.Contains(v, "o sort by close date") {
		t.Fatalf("key line does not offer the close date order:\n%s", v)
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
