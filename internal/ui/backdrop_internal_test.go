package ui

import "testing"

// A pop-up clears whole phrases of the frame beside it: a phrase it would cut goes, with
// a lone key or glyph it leaves, while two spaces or a rule end a phrase, so table
// columns and the pane divider stay.
func TestPhraseEdges(t *testing.T) {
	cases := []struct {
		row      string
		from, to int
		want     string // the row with the widened [from, to) blanked
	}{
		// The welcome: "a  add your Lead" cut at "your" goes whole, key and all.
		{"   a  add your Lead     x", 13, 20, "                        x"},
		// "▞ rolecall · first" cut in its middle: the glyph goes with the phrase.
		{"▞ rolecall · first run", 6, 22, "                      "},
		// Table columns two spaces apart: only the column cut goes.
		{"Platform Engineer  Fern Systems  SWE", 21, 25, "Platform Engineer                SWE"},
		// The pane divider stays whole, and no stub is taken across it.
		{"a │ some words here", 6, 10, "a │                "},
		// Phrases that end and start at the edges are whole already, and stay.
		{"done    words", 6, 8, "done    words"},
	}
	for _, c := range cases {
		from, to := phraseEdges(c.row, c.from, c.to)
		r := []rune(c.row)
		for i := from; i < to && i < len(r); i++ {
			r[i] = ' '
		}
		if got := string(r); got != c.want {
			t.Errorf("phraseEdges(%q, %d, %d) = [%d, %d): %q, want %q", c.row, c.from, c.to, from, to, got, c.want)
		}
	}
}
