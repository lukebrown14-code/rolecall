package ui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
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
	return nil, m.move(a, status, false)
}

// move appends a Status and says so in a toast. It returns the command that ends the
// moved row's flash.
func (m *model) move(a store.Application, status string, undo bool) tea.Cmd {
	if err := m.store.AppendStatus(a.Folder, status, m.now()); err != nil {
		m.err = err
		return nil
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
		return nil
	}
	name := moveName(a)
	if undo {
		m.toast = "Undid move " + name
		return m.startFlash(a.Folder)
	}
	m.undo = append(m.undo, undoMove{a.Folder, a.Status})
	m.toast, m.toastHint, m.movedTo = "Moved "+name+" → "+status, undoHint, status
	if m.hiddenByFilter(a.Folder) {
		m.toast += " (hidden by filter)"
	}
	if status == store.StatusDismissed {
		m.toast = "Dismissed " + name
	}
	if status == store.StatusOffer {
		m.toast = m.offerToast(a)
		// Land on the Application's new home so the celebration sits over its row, not
		// an empty list left behind.
		m.followAfterMove(a.Folder)
	}
	return m.startFlash(a.Folder)
}

// undoHint is the keys shown with the toast of every reversible action.
const undoHint = "u undo · esc hide"

// moveName is the company a toast names, falling back to the Application's title.
func moveName(a store.Application) string {
	if c := strings.TrimSpace(a.Company); c != "" {
		return c
	}
	return appName(a.Title)
}

// hiddenByFilter reports that a track or text filter is in force and the row is not among
// the rows now shown, so the toast can say where it went.
func (m model) hiddenByFilter(folder string) bool {
	if !m.filtered() {
		return false
	}
	return !slices.ContainsFunc(m.rows(), func(r store.Application) bool { return r.Folder == folder })
}

// followAfterMove puts the selection on folder's row in whichever screen and view now
// hold it, so a move that empties the current view lands on the Application's new home
// instead of an empty list. Filters are left alone: the moved Application was just on
// screen, so they cannot be hiding it.
func (m *model) followAfterMove(folder string) {
	for _, a := range m.apps {
		if a.Folder != folder {
			continue
		}
		screen, table := StatusLocation(a.Status)
		for i, d := range screenDefs {
			if d.name == screen {
				m.screen = i
			}
		}
		for i, t := range m.views() {
			if t == table {
				m.table = i
			}
		}
		m.detail = nil
		for i, r := range m.rows() {
			if r.Folder == folder {
				m.cursor = i
			}
		}
		return
	}
}

// flashFor is how long a moved row stays lit, so the eye can follow it.
const flashFor = 600 * time.Millisecond

// flashDone ends the flash it was started for; a later flash has a higher id and outlives it.
type flashDone struct{ id int }

// startFlash lights folder's row and returns the tick that puts it out.
func (m *model) startFlash(folder string) tea.Cmd {
	m.flashID++
	m.flash = folder
	id := m.flashID
	tick := m.tick
	if tick == nil {
		tick = tea.Tick
	}
	return tick(flashFor, func(time.Time) tea.Msg { return flashDone{id} })
}

// Ticker schedules a message after a delay: tea.Tick, or a fake in tests.
type Ticker func(time.Duration, func(time.Time) tea.Msg) tea.Cmd

// WithTicker replaces tea.Tick for the row flash, so tests fire it by hand.
func WithTicker(t Ticker) Option { return func(m *model) { m.tick = t } }

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
