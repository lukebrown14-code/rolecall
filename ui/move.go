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

// pendingMove is a move to Applied awaiting the Owner's answer to the not-Ready prompt.
type pendingMove struct {
	app    store.Application
	status string
}

// requestMove moves immediately, except that a non-Ready Application going to Applied
// first asks the Owner. A malformed Application skips the prompt so the store's error shows.
func (m *model) requestMove(a store.Application, status string) {
	if status == "Applied" && a.Malformed == nil && a.Documents != "Ready" {
		m.confirm = &pendingMove{a, status}
		return
	}
	m.move(a, status, false)
}

func (m *model) move(a store.Application, status string, undo bool) {
	if err := m.store.AppendStatus(a.Folder, status, m.now()); err != nil {
		m.err = err
		return
	}
	var sentErr error
	if status == "Applied" && !undo {
		sentErr = m.store.RecordSent(a.Folder)
	}
	// The append is final even when the copy failed, so the rows are refreshed either way.
	m.reload(a.Folder)
	if sentErr != nil {
		// Not a clean move: no success toast and nothing to undo, only the error.
		m.err, m.toast = sentErr, ""
		return
	}
	if !undo {
		m.undo = append(m.undo, undoMove{a.Folder, a.Status})
	}
	m.toast = fmt.Sprintf("Moved to %s · u to undo", status)
}

// statusBeforeDismissed is the Status the Owner restores to: the one before the last Dismissed
// entry of the log, skipping any run of Dismissed entries stacked at that point. A log with
// nothing earlier restores to Lead.
func statusBeforeDismissed(log []store.StatusEntry) string {
	i := len(log) - 1
	for i >= 0 && log[i].Status != "Dismissed" {
		i--
	}
	for i--; i >= 0; i-- {
		if log[i].Status != "Dismissed" {
			return log[i].Status
		}
	}
	return "Lead"
}
func (m model) moveView() string { return m.moveForm.View() }
