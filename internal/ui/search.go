package ui

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// searchText is what the text filter matches an Application against: Title and Company,
// or for a malformed one only its folder ("SWE/malformed"), whose parsed fields are
// partial and would match queries the Owner never meant for it.
func (m model) searchText(a store.Application) string {
	if a.Malformed != nil {
		return strings.TrimPrefix(m.rel(a.Folder), "jobs"+string(filepath.Separator))
	}
	return a.Title + " " + a.CompanyLabel()
}

// searchChip is the text filter while it is on, "/pl ×", shown on every screen so a
// filter never hides rows unnoticed; esc or a click on it clears it.
func (m model) searchChip(p pal) string {
	if m.text == "" {
		return ""
	}
	return p.Text(p.Fg).Bold(true).Render("/"+m.text) + p.Text(p.Dim).Render(" ×")
}

// withChip puts the search chip before a context row's right-hand text.
func (m model) withChip(p pal, right string) string {
	chip := m.searchChip(p)
	switch {
	case chip == "":
		return right
	case right == "":
		return chip
	}
	return chip + "   " + right
}

// searchElsewhere is where else the text filter has matches when this view has none:
// "1 in Applied" for another screen, "2 in Dismissed" for another view of this
// one, with the key that goes to the first of them. Empty when nothing else matches.
func (m model) searchElsewhere() (where []string, key, words string) {
	here := m.views()[m.table]
	for i, d := range screenDefs {
		if !d.hasTables {
			continue
		}
		n := 0
		for _, v := range d.tables {
			switch {
			case v == allView:
				continue // All is the sum of the others
			case i == m.screen && (here == allView || v == here):
				continue
			case i == m.screen:
				if c := m.count(i, v); c > 0 {
					where = append(where, fmt.Sprintf("%d in %s", c, v))
					// s goes to the next view only, so it is offered only for that one.
					if next := d.tables[(m.table+1)%len(d.tables)]; key == "" && v == next {
						key, words = "s", v
					}
				}
				continue
			}
			n += m.count(i, v)
		}
		if i != m.screen && n > 0 {
			where = append(where, fmt.Sprintf("%d in %s", n, d.name))
			if key == "" {
				key, words = strconv.Itoa(i+1), d.name
			}
		}
	}
	return where, key, words
}

// clearSearch is esc on a screen without rows of its own (Stats, Profile): it clears the
// text filter the tab bar's chip shows, and says so, as nothing else on that screen changes.
func (m *model) clearSearch() {
	if m.text == "" {
		return
	}
	m.toast = "Cleared search \"/" + m.text + "\""
	m.text = ""
}
