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

// undoMove is one move u can undo: the Application, the Status it goes back to, the
// Status the move left it at (an undo is refused once it is elsewhere), and
// whether the move created sent/, which the undo then sets aside. filterReason and
// filterNote are the Search rule a restore overruled, which the undo puts back.
// closeReason is the close_reason a Rejected or Withdrawn Application had before the
// move (a reopen removes it), which the undo puts back too.
type undoMove struct {
	folder, status, target   string
	sent                     bool
	filterReason, filterNote string
	closeReason              string
	// followUp marks a recorded follow-up (l), not a move: the undo puts back followedUp,
	// the followed_up day it replaced ("" removes the key).
	followUp   bool
	followedUp string
	// batch is a bulk move (bulk.go): one u puts back every move in it.
	batch []undoMove
}

// requestMove moves immediately, except that a non-Ready Application going to Applied
// first asks the Owner, returning the confirm mode. A malformed Application skips the
// prompt so the store's error shows.
func (m *model) requestMove(a store.Application, status string) (mode, tea.Cmd) {
	if status == store.StatusApplied && a.Malformed == nil && a.Documents != store.DocsReady {
		return &confirmMode{app: a, status: status}, nil
	}
	before := len(m.undo)
	cmd := m.move(a, status, false)
	if (status == store.StatusRejected || status == store.StatusWithdrawn) && m.err == nil && len(m.undo) > before {
		// The move happened; asking why it ended is optional, and esc skips it.
		return m.closeReasonPopup([]store.Application{a}, status), cmd
	}
	return nil, cmd
}

// move appends a Status and says so in a toast. It returns the command that ends the
// moved row's flash.
func (m *model) move(a store.Application, status string, undo bool) tea.Cmd {
	wrote, err := m.store.AppendStatus(a.Folder, status, m.now())
	if err != nil {
		m.err = err
		return nil
	}
	if !wrote {
		// The Application was already there (the rows were stale): nothing to undo.
		m.reload(a.Folder)
		m.toast, m.toastHint, m.toastKind = "Already "+status+" · "+moveName(a), "", toastNote
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
	hidden := m.hiddenByFilter(a.Folder)
	where := "Moved to " + status
	if status == store.StatusDismissed {
		where = "Dismissed"
	}
	if hidden {
		where += hiddenNote
	}
	if undo {
		// undoLast words the toast once the undo is whole (a Search rule put back, sent
		// set aside).
		m.toastKind = toastUndo
		return m.startFlash(a.Folder)
	}
	u := undoMove{folder: a.Folder, status: a.Status, target: status, sent: madeSent}
	if a.Filtered() && status != store.StatusDismissed {
		u.filterReason, u.filterNote = a.FilterReason, a.FilterNote
	}
	if endedStatus(a.Status) {
		u.closeReason = a.CloseReason
	}
	m.undo = append(m.undo, u)
	m.toast, m.toastHint, m.movedTo = where+" · "+name, undoHint, status
	if status == store.StatusOffer {
		m.toast, m.toastKind = m.offerToast(a), toastOffer
		if hidden {
			m.toast += hiddenNote
		}
		// Land on the Application's new home so the celebration sits over its row, not
		// an empty list left behind. A view that still shows it (Pipeline › All) is kept:
		// reload has put the cursor on the row's new place.
		if !hidden && !m.shown(a.Folder) {
			m.followAfterMove(a.Folder)
		}
	} else {
		m.offerFollow(a.Folder)
	}
	return m.startFlash(a.Folder)
}

// keyFollow goes to the Application a move or an undo just took off the screen: its new
// screen and view, the cursor on its row. The toast offers it for the one next key. g is
// taken (the top of a list and of the detail), and i is free on both.
const keyFollow = "i"

// descFollow is keyFollow's words in the toast.
const descFollow = "go to it"

// offerFollow offers keyFollow in the toast when the Application folder is now somewhere
// the Owner is not: not when the view on screen still lists it, nor when a filter would
// hide it at its new home (following would find no row).
func (m *model) offerFollow(folder string) {
	m.follow = ""
	if m.detail == nil && m.hasTables() && m.shown(folder) {
		return
	}
	probe := *m
	probe.followAfterMove(folder)
	if a, ok := probe.current(); !ok || a.Folder != folder {
		return
	}
	m.follow = folder
	offer := keyFollow + " " + descFollow
	switch {
	case m.toastHint == "":
		m.toastHint = offer
	case strings.HasPrefix(m.toastHint, "u undo"):
		m.toastHint = strings.Replace(m.toastHint, "u undo", "u undo · "+offer, 1)
	default:
		m.toastHint = offer + " · " + m.toastHint
	}
}

// whereNow is where an Application is listed, as the Owner finds it: "Drafts",
// "Pipeline › Applied", "Leads › Dismissed". A screen's own view is the screen alone.
func (m model) whereNow(folder string) string {
	a, ok := m.application(folder)
	if !ok {
		return ""
	}
	screen, table := appLocation(a)
	if table == screen || table == tableLeads {
		return screen
	}
	return screen + " › " + table
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

// hiddenNote follows where a toast says the Application went when a filter hides it there.
const hiddenNote = " (hidden by filter)"

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
// phantom stop and do not reset the age in Status. An undone first move of a Lead empties
// it: the Lead reads as never moved, as it did before. Otherwise it falls back to the
// whole log if the two ever disagree on the current Status.
func effectiveLog(a store.Application) []store.StatusEntry {
	log := reporting.EffectiveLog(a.StatusLog)
	if len(log) == 0 && a.Status == store.StatusLead {
		return nil
	}
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
	if len(log) == 0 {
		return nil
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
	if last.batch != nil {
		return m.undoBatch(last.batch)
	}
	a, ok := m.application(last.folder)
	if !ok {
		return nil
	}
	if last.followUp {
		return m.undoFollowUp(a, last.followedUp)
	}
	// Read the file itself: the rows can lag behind another writer.
	if now, err := m.store.Rescan(a.Folder); err == nil && now != nil && now.Status != last.target {
		m.undo = m.undo[:len(m.undo)-1]
		m.reload(a.Folder)
		m.toast, m.toastHint, m.toastKind = "Not undone · "+moveName(a)+" changed since the move", "", toastNote
		return nil
	}
	cmd := m.move(a, last.status, true)
	if m.err != nil {
		return cmd
	}
	m.undo = m.undo[:len(m.undo)-1]
	if last.filterReason != "" {
		if err := m.store.SetFilter(a.Folder, last.filterReason, last.filterNote); err != nil {
			m.err = err
			return cmd
		}
		m.reload(m.cursorFolder())
	}
	if last.closeReason != "" {
		if err := m.store.SetCloseReason(a.Folder, last.closeReason); err != nil {
			m.err = err
			return cmd
		}
		m.reload(m.cursorFolder())
	}
	if last.sent {
		if _, err := m.store.SetAsideSent(a.Folder, m.now()); err != nil {
			m.err = err
			return cmd
		}
		m.reload(m.cursorFolder())
	}
	m.toast, m.toastHint = m.undoToast(a, last), ""
	m.offerFollow(a.Folder)
	return cmd
}

// undoToast says what an undo did in the Owner's words, where the Application went first
// so a short footer cuts the name: "Undone · back to Draft · on Drafts · Backend
// Engineer (Numbat Health)". A set-aside sent copy is named as such, never by its folder.
func (m model) undoToast(a store.Application, u undoMove) string {
	toast := "Undone · back to " + u.status
	if where := m.whereNow(a.Folder); where != "" {
		toast += " · on " + where
	}
	if m.hiddenByFilter(a.Folder) {
		toast += hiddenNote
	}
	toast += " · " + moveName(a)
	if u.sent {
		toast += " · its sent copy set aside"
	}
	return toast
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
		screen, table := appLocation(a)
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
