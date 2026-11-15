package ui

import (
	"slices"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/reporting"
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

// undoMove is one move u can undo: the Application, the Status it goes back to, and
// whether the move created sent/, which the undo then sets aside.
type undoMove struct {
	folder, status string
	sent           bool
}

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
	madeSent := false
	if status == store.StatusApplied && !undo {
		madeSent = !a.HasSent
		sentErr = m.store.RecordSent(a.Folder)
	}
	// The append is final even when the copy failed, so the rows are refreshed either way.
	m.reload(a.Folder)
	if sentErr != nil {
		// Not a clean move: no success toast and nothing to undo, only the error.
		m.err, m.toast = sentErr, ""
		return nil
	}
	// Each toast leads with where the Application went, then which one it was, so a
	// short footer cuts the Company first and never the Status.
	name := moveName(a)
	const hiddenNote = " (hidden by filter)"
	hidden := m.hiddenByFilter(a.Folder)
	where := "Moved to " + status
	switch {
	case undo:
		where = "Undone · back to " + status
	case status == store.StatusDismissed:
		where = "Dismissed"
	}
	if hidden {
		where += hiddenNote
	}
	if undo {
		m.toast, m.toastKind = where+" · "+name, toastUndo
		return m.startFlash(a.Folder)
	}
	m.undo = append(m.undo, undoMove{a.Folder, a.Status, madeSent})
	m.toast, m.toastHint, m.movedTo = where+" · "+name, undoHint, status
	if status == store.StatusOffer {
		m.toast, m.toastKind = m.offerToast(a), toastOffer
		if hidden {
			m.toast += hiddenNote
		}
		// Land on the Application's new home so the celebration sits over its row, not
		// an empty list left behind. A view that still shows it (Applied › All) is kept:
		// reload has put the cursor on the row's new place.
		if !hidden && !m.shown(a.Folder) {
			m.followAfterMove(a.Folder)
		}
	}
	return m.startFlash(a.Folder)
}

// toastKind says how the feedback line draws a success toast, so the drawing does not
// depend on the toast's wording.
type toastKind int

const (
	toastPlain toastKind = iota // a ✓ in the ok colour
	toastUndo                   // an undo: ✓ like any success (↩ is Withdrawn)
	toastOffer                  // a move to Offer: the gold ★ celebration
	toastNote                   // nothing happened (e.g. nothing to undo): no ✓, muted
)

// undoHint is the keys shown with the toast of every reversible action.
const undoHint = "u undo · esc hide"

// moveName is the Application as a toast names it, "Platform Engineer (Fern Systems)":
// the title says which one, as several can share a Company.
func moveName(a store.Application) string {
	name := appName(a.Title)
	if c := strings.TrimSpace(a.Company); c != "" {
		name += " (" + c + ")"
	}
	return name
}

// effectiveLog is the Status log the rows, timeline and ages read: undone moves taken out
// (reporting.EffectiveLog), as Stats reads it, so a fat-fingered move and its undo leave no
// phantom stop and do not reset the age in Status. It falls back to the whole log if the
// two ever disagree on the current Status.
func effectiveLog(a store.Application) []store.StatusEntry {
	log := reporting.EffectiveLog(a.StatusLog)
	if len(log) == 0 || log[len(log)-1].Status != a.Status {
		return a.StatusLog
	}
	return log
}

// lastChange is when the Application reached its current Status, ignoring undone moves:
// the store's LastChange unless an undo took entries out.
func lastChange(a store.Application) *store.StatusEntry {
	log := effectiveLog(a)
	if len(log) == len(a.StatusLog) {
		return a.LastChange
	}
	e := log[len(log)-1]
	return &e
}

// appliedDate is the Application's first Applied entry that was not undone, or nil: the
// store's AppliedDate unless an undo took entries out.
func appliedDate(a store.Application) *store.StatusEntry {
	log := effectiveLog(a)
	if len(log) == len(a.StatusLog) {
		return a.AppliedDate
	}
	for _, e := range log {
		if e.Status == store.StatusApplied {
			return &e
		}
	}
	return nil
}

// shown reports whether the active view lists folder's row.
func (m model) shown(folder string) bool {
	return slices.ContainsFunc(m.rows(), func(r store.Application) bool { return r.Folder == folder })
}

// undoLast is u: it moves the last moved Application back, and sets aside the sent/ that
// move created, so a later real Apply freezes what is really sent.
func (m *model) undoLast() tea.Cmd {
	if len(m.undo) == 0 {
		m.toast, m.toastKind = "Nothing to undo", toastNote
		return nil
	}
	last := m.undo[len(m.undo)-1]
	a, ok := m.application(last.folder)
	if !ok {
		return nil
	}
	cmd := m.move(a, last.status, true)
	if m.err != nil {
		return cmd
	}
	m.undo = m.undo[:len(m.undo)-1]
	if last.sent {
		name, err := m.store.SetAsideSent(a.Folder, m.now())
		if err != nil {
			m.err = err
			return cmd
		}
		m.reload(m.cursorFolder())
		m.toast += " · sent/ kept as " + name + "/"
	}
	return cmd
}

// hiddenByFilter reports that a track or text filter is in force and the row is not among
// the rows now shown, so the toast can say where it went. Off the lists (an undo from
// Stats or Profile) no rows are shown, so none is hidden.
func (m model) hiddenByFilter(folder string) bool {
	if !m.filtered() || !m.hasTables() {
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
