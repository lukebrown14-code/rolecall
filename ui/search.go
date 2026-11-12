package ui

import (
	"path/filepath"
	"strings"

	"github.com/lukebrown14-code/rolecall/store"
)

// searchText is what the text filter matches an Application against: Title and Company,
// or for a malformed one only its folder ("SWE/malformed"), whose parsed fields are
// partial and would match queries the Owner never meant for it.
func (m model) searchText(a store.Application) string {
	if a.Malformed != nil {
		return strings.TrimPrefix(m.rel(a.Folder), "jobs"+string(filepath.Separator))
	}
	return a.Title + " " + a.Company
}

// searchChip is the text filter while it is on, "/pl ×", shown on every screen so a
// filter never hides rows unnoticed; esc or a click on it clears it.
func (m model) searchChip(p pal) string {
	if m.text == "" {
		return ""
	}
	return p.text(p.accent).Bold(true).Render("/"+m.text) + p.text(p.dim).Render(" ×")
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
