package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// An empty screen says what is missing in one muted line and the next key to press, as a
// key chip. A hint is offered only when the registry lists its key here, so it is never
// wrong: a key that does nothing in this context is left out.

// emptyHint is one next-key hint: the key and the words beside it.
type emptyHint struct{ key, words string }

// addLeadHint is the hint every empty list shares.
var addLeadHint = emptyHint{"a", "add a Lead"}

// keyAvailable reports whether the registry lists key in the current context.
func (m model) keyAvailable(key string) bool {
	for _, k := range m.screenKeys() {
		if k.Help().Key == key {
			return true
		}
	}
	return false
}

// availableHints drops the hints whose key is not listed here.
func (m model) availableHints(hints ...emptyHint) []emptyHint {
	var out []emptyHint
	for _, h := range hints {
		if m.keyAvailable(h.key) {
			out = append(out, h)
		}
	}
	return out
}

// emptyViewNoun completes "No …" for a view.
func emptyViewNoun(table string) string {
	switch table {
	case tableLeads:
		return "Leads"
	case tableDismissed:
		return "dismissed Leads"
	case tableDraft:
		return "Drafts"
	case tableApplied:
		return "Applied Applications"
	case tableInterviewing:
		return "Interviewing Applications"
	case tableOffer:
		return "Offers"
	case tableClosed:
		return "closed Applications"
	}
	return "Applications"
}

// emptyViewLine is what an empty view says and the keys that help: change the Track when
// one filters, change the Status filter, add a Lead.
func (m model) emptyViewLine(table string) (string, []emptyHint) {
	msg := "No " + emptyViewNoun(table)
	hints := []emptyHint{{"s", "change filter"}, addLeadHint}
	switch {
	case m.track > 0:
		msg += " in " + m.trackName()
		hints = append([]emptyHint{{"t", "change Track"}}, hints...)
	case table != tableLeads && table != tableDismissed:
		msg += " yet"
	}
	return msg, m.availableHints(hints...)
}

// emptySearchLine is the line for a search with no matches.
func (m model) emptySearchLine() (string, []emptyHint) {
	return "No Applications match \"" + m.text + "\"", m.availableHints(emptyHint{"esc", "clear search"})
}

// emptyLines lays a message and its hints out as muted text in rows no wider than w,
// breaking only between hints so a key chip stays beside its words.
func (m model) emptyLines(p pal, w int, msg string, hints []emptyHint) []string {
	w = max(1, w)
	sep, sepW := p.Text(p.Dim).Render(" · "), 3
	atoms := []string{p.Text(p.Dim).Render(msg)}
	for _, h := range hints {
		atoms = append(atoms, p.Key().Render(h.key)+p.Text(p.Dim).Render(" "+h.words))
	}
	var lines []string
	cur, curW := "", 0
	for _, a := range atoms {
		aw := xansi.StringWidth(a)
		if cur != "" && curW+sepW+aw > w {
			lines, cur, curW = append(lines, cur), "", 0
		}
		if cur != "" {
			cur, curW = cur+sep, curW+sepW
		}
		cur, curW = cur+a, curW+aw
	}
	lines = append(lines, cur)
	// A message longer than the row wraps on word boundaries.
	for i, l := range lines {
		if xansi.StringWidth(l) > w {
			lines[i] = lipgloss.NewStyle().Width(w).Render(l)
		}
	}
	return strings.Split(strings.Join(lines, "\n"), "\n")
}

// emptyBody places the empty lines in a body: centred at wide sizes, top-aligned and
// indented at narrow ones.
func (m model) emptyBody(p pal, w, h int, msg string, hints []emptyHint) string {
	if m.layout().wide() {
		lines := m.emptyLines(p, w-2*gutter, msg, hints)
		return lipgloss.Place(w, h, lipgloss.Center, lipgloss.Center, strings.Join(lines, "\n"))
	}
	pad := strings.Repeat(" ", gutter)
	lines := m.emptyLines(p, w-gutter-rightMargin, msg, hints)
	return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, pad+strings.Join(lines, "\n"+pad))
}
