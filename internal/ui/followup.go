package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
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

// followUpStatus reports a Status that can be Follow-up due: Applied or Interviewing.
func followUpStatus(status string) bool {
	return status == store.StatusApplied || status == store.StatusInterviewing
}

// followUpFrom is the day Follow-up due counts from: the later of the last Status change
// and the last recorded follow-up. ok is false when neither can be read.
func followUpFrom(a store.Application) (from time.Time, ok bool) {
	if e := lastChange(a); e != nil {
		from, ok = parseDay(e.At)
	}
	if t, read := parseDay(a.FollowedUp); read && (!ok || t.After(from)) {
		from, ok = t, true
	}
	return from, ok
}

// followUpQuiet is how long a sent Application has been quiet, as a span ("6w"): the days
// since followUpFrom, the same day followUpDaysPast counts from, so the row's "⚑ 6w" and
// the card's "due · …" can never disagree. Always a span, since "quiet Feb 2026" would read
// as a date; "" when neither date can be read.
func (m model) followUpQuiet(a store.Application) string {
	from, ok := followUpFrom(a)
	if !ok {
		return ""
	}
	return span(max(int(day(m.now()).Sub(from).Hours()/hoursPerDay), 0))
}

// followedUpLater reports a recorded follow-up later than the last Status change, so it
// is what the Follow-up due count runs from. A follow-up before a later move is history.
func followedUpLater(a store.Application) bool {
	t, ok := parseDay(a.FollowedUp)
	if !ok {
		return false
	}
	if e := lastChange(a); e != nil {
		if c, read := parseDay(e.At); read && !t.After(c) {
			return false
		}
	}
	return true
}

// recordFollowUp is l: today becomes the Application's followed_up day, with u to take it
// back. A second l the same day changes nothing and says so.
func (m *model) recordFollowUp(a store.Application) tea.Cmd {
	today := day(m.now()).Format(time.DateOnly)
	if a.FollowedUp == today {
		m.toast, m.toastKind = "Follow-up already recorded today · "+moveName(a), toastNote
		return nil
	}
	if err := m.store.SetFollowedUp(a.Folder, today); err != nil {
		m.err = err
		return nil
	}
	m.reload(a.Folder)
	m.undo = append(m.undo, undoMove{folder: a.Folder, followUp: true, followedUp: a.FollowedUp})
	next := day(m.now()).AddDate(0, 0, m.followUpDays)
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
	m.reload(a.Folder)
	m.toast, m.toastKind = "Undone · follow-up not recorded · "+moveName(a), toastUndo
	return m.startFlash(a.Folder)
}

// followedUpRow is the card's "FOLLOW-UP  sent 3d ago · next due in 11d" once a
// follow-up is recorded and not yet due again; "" otherwise (NEXT says when it is due, and
// a booked next step holds it, so NEXT STEP says what comes next).
func (m model) followedUpRow(p pal, a store.Application, w int) string {
	if a.Malformed != nil || !followUpStatus(a.Status) || !followedUpLater(a) || nextStepBooked(a, m.now()) {
		return ""
	}
	past, due := followUpDaysPast(a, m.now(), m.followUpDays)
	if due {
		return ""
	}
	sent, _ := ago(a.FollowedUp, m.now())
	return p.cardRow(followUpLabel, p.Text(p.Fg).Render("sent "+sent+" · next due in "+span(-past)), "", "", w)
}
