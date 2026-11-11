package ui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

var statusLetters = map[string]string{store.StatusLead: "l", store.StatusDraft: "d", store.StatusApplied: "a", store.StatusInterviewing: "i", store.StatusOffer: "o", store.StatusRejected: "r", store.StatusWithdrawn: "w", store.StatusDismissed: "x"}

// MoveOptions returns likely next Statuses first, excluding the current Status.
func MoveOptions(from, previous string) []string {
	priorities := map[string][]string{
		store.StatusLead: {store.StatusDraft, store.StatusApplied, store.StatusDismissed}, store.StatusDraft: {store.StatusApplied, store.StatusLead, store.StatusDismissed},
		store.StatusApplied:      {store.StatusInterviewing, store.StatusRejected, store.StatusWithdrawn, store.StatusOffer},
		store.StatusInterviewing: {store.StatusOffer, store.StatusRejected, store.StatusWithdrawn, store.StatusApplied},
		store.StatusOffer:        {store.StatusWithdrawn, store.StatusRejected}, store.StatusRejected: {store.StatusInterviewing, store.StatusApplied},
		store.StatusWithdrawn: {store.StatusInterviewing, store.StatusApplied}, store.StatusDismissed: {previous},
	}
	seen := map[string]bool{from: true, "": true}
	var out []string
	for _, s := range append(priorities[from], store.Statuses...) {
		if !seen[s] {
			out = append(out, s)
			seen[s] = true
		}
	}
	return out
}

type undoMove struct{ folder, status string }

// requestMove moves immediately, except that a non-Ready Application going to Applied
// first asks the Owner, returning the confirm mode. A malformed Application skips the
// prompt so the store's error shows.
func (m *model) requestMove(a store.Application, status string) (mode, tea.Cmd) {
	if status == store.StatusApplied && a.Malformed == nil && a.Documents != store.DocsReady {
		return &confirmMode{app: a, status: status}, nil
	}
	m.move(a, status, false)
	return nil, nil
}

func (m *model) move(a store.Application, status string, undo bool) {
	if err := m.store.AppendStatus(a.Folder, status, m.now()); err != nil {
		m.err = err
		return
	}
	var sentErr error
	if status == store.StatusApplied && !undo {
		sentErr = m.store.RecordSent(a.Folder)
	}
	// The append is final even when the copy failed, so the rows are refreshed either way.
	m.reload(a.Folder)
	if sentErr != nil {
		// Not a clean move: no success toast and nothing to undo, only the error.
		m.err, m.toast = sentErr, ""
		return
	}
	if undo {
		m.toast = fmt.Sprintf("Undone · %s back to %s", a.Title, status)
		return
	}
	m.undo = append(m.undo, undoMove{a.Folder, a.Status})
	m.toast, m.toastHint = fmt.Sprintf("%s → %s", a.Title, status), "u undo"
}

// statusBeforeDismissed is the Status the Owner restores to: the one before the last Dismissed
// entry of the log, skipping any run of Dismissed entries stacked at that point. A log with
// nothing earlier restores to Lead.
func statusBeforeDismissed(log []store.StatusEntry) string {
	i := len(log) - 1
	for i >= 0 && log[i].Status != store.StatusDismissed {
		i--
	}
	for i--; i >= 0; i-- {
		if log[i].Status != store.StatusDismissed {
			return log[i].Status
		}
	}
	return store.StatusLead
}
