package ui

import (
	"fmt"
	"slices"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
)

// The filter row: the view (a status filter) and the Track filter on one line,
// "VIEW s [All 9] Applied 1 Interviewing 2 │ TRACK t [All 9] SWE 8 Platform 1". Each
// count is the rows that option would show, so the chosen ones match the list.

// noTrack is how the Track filter names Applications with no track.
const noTrack = "No track"

// filterTracks is the Track filter's options after All: the workspace.json Tracks in
// their order, then every other Track an Application is on (a legacy "Unknown", or one
// renamed or removed from workspace.json) in name order, so the options add up to All.
// m.track indexes it, 1-based.
func (m model) filterTracks() []string {
	listed := make(map[string]bool, len(m.tracks))
	for _, t := range m.tracks {
		listed[t] = true
	}
	var other []string
	for _, a := range m.apps {
		if !listed[a.Track] {
			listed[a.Track] = true
			other = append(other, a.Track)
		}
	}
	slices.Sort(other)
	return append(slices.Clone(m.tracks), other...)
}

// selectedTrack is the Track the filter is on; ok is false on All, or when the chosen
// off-list Track has since gone. A workspace.json Track needs no scan of the rows.
func (m model) selectedTrack() (track string, ok bool) {
	switch {
	case m.track <= 0:
		return "", false
	case m.track <= len(m.tracks):
		return m.tracks[m.track-1], true
	}
	if all := m.filterTracks(); m.track <= len(all) {
		return all[m.track-1], true
	}
	return "", false
}

// trackLabel is a Track as the filter shows it: an empty one is "No track".
func trackLabel(track string) string {
	if track == "" {
		return noTrack
	}
	return track
}

// filterSpan is where one option sits on the filter row, for a click.
type filterSpan struct {
	from, to int  // the cells it covers, from inclusive, to exclusive
	track    bool // a Track option, as opposed to a view
	idx      int  // the view, or the Track (0 is All)
}

// filterPart is one selector drawn: its text and its options' spans, relative to its start.
type filterPart struct {
	text  string
	spans []filterSpan
}

// add appends s to the part; with idx >= 0 it records s as that option.
func (f *filterPart) add(s string, track bool, idx int) {
	at := xansi.StringWidth(f.text)
	f.text += s
	if idx >= 0 {
		f.spans = append(f.spans, filterSpan{from: at, to: at + xansi.StringWidth(s), track: track, idx: idx})
	}
}

// options draws a selector: the label and key, then each option with its count, the chosen
// one as a chip. Compact keeps only the chosen option; collapsed is "s:All".
func (m model) options(p pal, label, key string, names []string, counts []int, chosen int, form trackForm, track bool) filterPart {
	var f filterPart
	k := ""
	if key != "" {
		k = p.Key().Render(key)
	}
	switch form {
	case trackCollapsed:
		if k != "" {
			f.add(k+p.Text(p.Dim).Render(":"), false, -1)
		}
		f.add(p.Text(p.Dim).Render(names[chosen]), track, chosen)
		return f
	case trackCompact:
		if k != "" {
			f.add(k+" ", false, -1)
		}
		f.add(p.SelChip(fmt.Sprintf("%s %d", names[chosen], counts[chosen])), track, chosen)
		if len(names) > 1 {
			f.add(p.Text(p.Dim).Render("▸"), false, -1)
		}
		return f
	}
	f.add(p.Text(p.Dim).Render(label+" "), false, -1)
	if k != "" {
		f.add(k+" ", false, -1)
	}
	for i, name := range names {
		// Options sit two cells apart, as on the tab bar. The chip's fill takes one of
		// them, so the chosen option keeps the same gap as the rest.
		if i > 0 && i != chosen && i-1 != chosen {
			f.add("  ", false, -1)
		} else if i > 0 {
			f.add(" ", false, -1)
		}
		text := fmt.Sprintf("%s %d", name, counts[i])
		if i == chosen {
			f.add(p.SelChip(text), track, i)
			continue
		}
		f.add(p.Text(p.Fg).Render(name)+p.Text(p.Dim).Render(fmt.Sprintf(" %d", counts[i])), track, i)
	}
	return f
}

// statusPart is the status filter: the screen's views and how many rows each shows.
func (m model) statusPart(p pal, form trackForm) filterPart {
	views := m.views()
	counts := make([]int, len(views))
	for i, v := range views {
		counts[i] = m.count(m.screen, v)
	}
	key := "s"
	if len(views) <= 1 {
		key = "" // one view: nothing to cycle
	}
	return m.options(p, "VIEW", key, views, counts, m.table, form, false)
}

// trackPart is the Track filter: All and each Track (see filterTracks), with the rows each
// would show in this view. Empty when the Workspace has no Tracks.
func (m model) trackPart(p pal, form trackForm) filterPart {
	if !m.trackApplies() {
		return filterPart{}
	}
	names := []string{allView}
	for _, t := range m.filterTracks() {
		names = append(names, trackLabel(t))
	}
	counts := make([]int, len(names))
	for i := range names {
		counts[i] = m.trackTotal(i)
	}
	return m.options(p, "TRACK", "t", names, counts, m.track, form, true)
}

// filterForms is every fit of the filter row, widest first: the status form, then the
// Track form.
var filterForms = [][2]trackForm{
	{trackFull, trackFull},
	{trackFull, trackCompact},
	{trackCompact, trackCompact},
	{trackCompact, trackCollapsed},
	{trackCollapsed, trackCollapsed},
}

// filterLine is the filter row's left side in the widest form that leaves right its room,
// with every option's span measured from the row's first cell.
func (m model) filterLine(p pal, right string) (string, []filterSpan) {
	w := m.layout().w
	pad := strings.Repeat(" ", inset)
	var line string
	var spans []filterSpan
	for _, f := range filterForms {
		line, spans = pad, nil
		for _, part := range []filterPart{m.statusPart(p, f[0]), m.trackPart(p, f[1])} {
			if part.text == "" {
				continue
			}
			if line != pad {
				line += p.Text(p.Rule).Render(" │ ")
			}
			at := xansi.StringWidth(line)
			for _, s := range part.spans {
				s.from, s.to = s.from+at, s.to+at
				spans = append(spans, s)
			}
			line += part.text
		}
		need := xansi.StringWidth(line)
		if right != "" {
			need += gutter + xansi.StringWidth(right)
		}
		if need <= w-rightMargin {
			break
		}
	}
	return line, spans
}

// filterRow is the table screens' one context line: the filters on the left, the search
// chip and the shown-of-total count on the right.
func (m model) filterRow(p pal) string {
	right := m.withChip(p, m.scopeLine(p))
	left, _ := m.filterLine(p, right)
	return contextRow(left, right, m.layout().w)
}

// filterAt is the option under column x of the filter row.
func (m model) filterAt(x int) (filterSpan, bool) {
	p := m.pal()
	_, spans := m.filterLine(p, m.withChip(p, m.scopeLine(p)))
	for _, s := range spans {
		if x >= s.from && x < s.to {
			return s, true
		}
	}
	return filterSpan{}, false
}

// pickFilter shows the option a click landed on.
func (m model) pickFilter(s filterSpan) model {
	folder := m.cursorFolder()
	if s.track {
		m.track = s.idx
	} else {
		m.table = s.idx
	}
	m.cursor = 0
	if s.track {
		m.keepCursor(folder)
	}
	return m.syncDetail()
}
