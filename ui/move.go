package ui

import (
	"fmt"
	"github.com/lukebrown14-code/rolecall/store"
)

var statusLetters = map[string]string{"Lead": "l", "Draft": "d", "Applied": "a", "Interviewing": "i", "Offer": "o", "Rejected": "r", "Withdrawn": "w", "Dismissed": "x"}

// MoveOptions returns likely next Statuses first, excluding the current Status.
func MoveOptions(from, previous string) []string {
	priorities := map[string][]string{
		"Lead": {"Draft", "Applied", "Dismissed"}, "Draft": {"Applied", "Lead", "Dismissed"},
		"Applied":      {"Interviewing", "Rejected", "Withdrawn", "Offer"},
		"Interviewing": {"Offer", "Rejected", "Withdrawn", "Applied"},
		"Offer":        {"Withdrawn", "Rejected"}, "Rejected": {"Interviewing", "Applied"},
		"Withdrawn": {"Interviewing", "Applied"}, "Dismissed": {previous},
	}
	seen := map[string]bool{from: true, "": true}
	var out []string
	for _, s := range append(priorities[from], "Lead", "Draft", "Applied", "Interviewing", "Offer", "Rejected", "Withdrawn", "Dismissed") {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

type undoMove struct{ folder, status string }

func (m *model) move(a store.Application, status string, undo bool) {
	if err := m.store.AppendStatus(a.Folder, status, m.now()); err != nil {
		m.err = err
		return
	}
	if !undo {
		m.undo = append(m.undo, undoMove{a.Folder, a.Status})
	}
	m.apps, m.err = m.store.Scan()
	m.toast = fmt.Sprintf("Moved to %s · u to undo", status)
}
func (m model) moveView() string { return m.moveForm.View() }
