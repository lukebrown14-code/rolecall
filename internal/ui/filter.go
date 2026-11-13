package ui

import (
	"fmt"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
)

// The filter row: the status filter and the Track filter on one line,
// "STATUS s [All 9] Applied 1 Interviewing 2 │ TRACK t [All 9] SWE 8 Platform 1". Each
// count is the rows that option would show, so the chosen ones match the list.

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
		if i > 0 {
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
	return m.options(p, "STATUS", key, views, counts, m.table, form, false)
}

// trackPart is the Track filter: All and each Track, with the rows each would show in this
// view. Empty when the Workspace has no Tracks.
func (m model) trackPart(p pal, form trackForm) filterPart {
	if !m.trackApplies() {
		return filterPart{}
	}
	names := append([]string{allView}, m.tracks...)
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
