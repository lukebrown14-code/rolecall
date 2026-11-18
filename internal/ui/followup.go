package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// Recording a follow-up: l on an Applied or Interviewing Application writes today as its
// followed_up day (SCHEMA.md) without changing its Status. Follow-up due then counts from
// that day, so the ⚑ clears and comes back follow_up_days later; Stats, which reads only
// the status_log, is unaffected. u takes it back like a move.
const (
	keyFollowedUp  = "l"
	descFollowedUp = "followed up"
)

// followUpQuiet is how long a sent Application has been quiet, as a span ("6w"): the days
// since schedule.FollowUpFrom, the same day schedule.FollowUpDaysPast counts from, so the row's "⚑ 6w" and
// the card's "due · …" can never disagree. Always a span, since "quiet Feb 2026" would read
// as a date; "" when neither date can be read.
func (m model) followUpQuiet(a store.Application) string {
	quiet, ok := schedule.QuietDays(a, m.now())
	if !ok {
		return ""
	}
	return span(quiet)
}

// recordFollowUp is l: today becomes the Application's followed_up day, with u to take it
// back. A second l the same day changes nothing and says so.
func (m *model) recordFollowUp(a store.Application) tea.Cmd {
	today := store.Today(m.now()).Format(time.DateOnly)
	if a.FollowedUp == today {
		m.toast, m.toastKind = "Follow-up already recorded today · "+moveName(a), toastNote
		return nil
	}
	if err := m.store.SetFollowedUp(a.Folder, today); err != nil {
		m.err = err
		return nil
	}
	m.reload(a.Folder, a.Folder)
	m.undo = append(m.undo, undoMove{folder: a.Folder, followUp: true, followedUp: a.FollowedUp})
	next := store.Today(m.now()).AddDate(0, 0, m.followUpDays)
	m.toast, m.toastHint = "Followed up · "+moveName(a)+" · next due "+shortDate(next, m.now()), undoHint
	return m.startFlash(a.Folder)
}

// undoFollowUp puts back the followed_up day an l replaced, or removes the key it added.
func (m *model) undoFollowUp(a store.Application, was string) tea.Cmd {
	if err := m.store.SetFollowedUp(a.Folder, was); err != nil {
		m.err = err
		return nil
	}
	m.undo = m.undo[:len(m.undo)-1]
	m.reload(a.Folder, a.Folder)
	m.toast, m.toastKind = "Undone · follow-up not recorded · "+moveName(a), toastUndo
	return m.startFlash(a.Folder)
}

// followedUpRow is the card's "FOLLOW-UP  sent 3d ago · next due in 11d" once a
// follow-up is recorded and not yet due again; "" otherwise (NEXT says when it is due, and
// a booked next step holds it, so NEXT STEP says what comes next).
func (m model) followedUpRow(p pal, a store.Application, w int) string {
	if a.Malformed != nil || !schedule.FollowUpStatus(a.Status) || !schedule.FollowedUpLater(a, m.now().Location()) || schedule.NextStepBooked(a, m.now()) {
		return ""
	}
	past, due := schedule.FollowUpDaysPast(a, m.now(), m.followUpDays)
	if due {
		return ""
	}
	sent, _ := ago(a.FollowedUp, m.now())
	return p.cardRow(followUpLabel, p.Text(p.Fg).Render("sent "+sent+" · next due in "+span(-past)), "", "", w)
}
