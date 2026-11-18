package ui

import (
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// rowNextStepWhat caps the next step's words on a row, so the facts after it keep room.
const rowNextStepWhat = 18

// wordToday is a next step due today, as its row says it.
const wordToday = "today"

// rowNextStep is an Applied, Interviewing or Offer Application's next step as its row's
// meta line words it, right after the Company: "▲ next tomorrow 14:30, Phone screen". The
// ▲ leads, so a narrow row that cuts the words keeps the urgency, and NO_COLOR keeps it
// too. Today and tomorrow are said as words; a later day is the short date, a passed one
// "next was 3 Oct" (struck through).
func (m model) rowNextStep(a store.Application) (string, closeState) {
	switch a.Status {
	case store.StatusApplied, store.StatusInterviewing, store.StatusOffer:
	default:
		return "", closeNone
	}
	ns := a.NextStep
	if ns == nil {
		return "", closeNone
	}
	t, days, ok := daysFrom(ns.Day(), m.now())
	when := ns.Day()
	switch {
	case ok && days == 0:
		when = wordToday
	case ok && days == 1:
		when = "tomorrow"
	case ok:
		when = shortDate(t, m.now())
	}
	if clock := ns.Time(); clock != "" {
		when += " " + clock
	}
	text, state := "next "+when, closeLater
	switch {
	case !ok:
	case days < 0:
		text, state = "next was "+when, closePast
	case days <= urgentDays:
		text, state = urgentMark+" next "+when, closeUrgent
	case days <= soonDays:
		state = closeSoon
	}
	if ns.What != "" {
		text += ", " + xansi.Truncate(ns.What, rowNextStepWhat, "…")
	}
	return text, state
}

// nextStepInfo words an Application's next step with its urgency, as closeInfo does a close
// date: "3 Dec 10:30 · Phone screen", with ▲ within urgentDays and "was 3 Dec" once passed.
// The urgency state is closeState, so a row and the card draw it with CloseStyle.
func (m model) nextStepInfo(ns *store.NextStep) (string, closeState) {
	if ns == nil {
		return "", closeNone
	}
	t, days, ok := daysFrom(ns.Day(), m.now())
	when := ns.Day()
	if ok {
		when = shortDate(t, m.now())
	}
	if clock := ns.Time(); clock != "" {
		when += " " + clock
	}
	text, state := when, closeLater
	switch {
	case !ok:
	case days < 0:
		text, state = "was "+when, closePast
	case days <= urgentDays:
		text, state = when+" "+urgentMark, closeUrgent
	case days <= soonDays:
		state = closeSoon
	}
	if ns.What != "" {
		text += " · " + ns.What
	}
	return text, state
}

// contactRows are the card's PEOPLE rows (an Application's contacts, never the Owner's Contact): one line per person, a long one wrapped
// under itself.
func (m model) contactRows(p pal, a store.Application, w int) []string {
	var rows []string
	indent := strings.Repeat(" ", cardLabelW)
	for i, c := range a.Contacts {
		label := ""
		if i == 0 {
			label = "PEOPLE"
		}
		lines := wrapWords(c.Label(), max(1, w-cardLabelW))
		if i == 0 {
			rows = append(rows, p.cardRow(label, p.Text(p.Fg).Render(lines[0]), "", "", w))
		} else {
			rows = append(rows, indent+p.Text(p.Fg).Render(lines[0]))
		}
		for _, l := range lines[1:] {
			rows = append(rows, indent+p.Text(p.Fg).Render(l))
		}
	}
	return rows
}

// nextStepRow is the card's NEXT STEP row, with the e key that edits it. Once the
// Application is closed or dismissed its next step no longer matters, but it stays in
// meta.yml, so the row stays too, struck through and muted with "· closed": nothing is kept
// out of sight. e on the CLOSED row (the same Edit form) clears it.
func (m model) nextStepRow(p pal, a store.Application, w int, keep func(string) bool) []string {
	text, s := m.nextStepInfo(a.NextStep)
	if s == closeNone {
		return nil
	}
	if !openStatus(a.Status) {
		value := p.Text(p.Dim).Strikethrough(true).Render(text) + p.Text(p.Dim).Render(" · closed")
		return []string{p.cardRow("NEXT STEP", value, "", "", w)}
	}
	key, action := "e", descEdit
	if !keep(key) {
		key, action = "", ""
	}
	return []string{p.cardRow("NEXT STEP", p.CloseStyle(s, p.Text, p.Dim).Render(text), key, action, w)}
}

// closedRow is the card's CLOSED row: why a Rejected or Withdrawn Application ended, in
// words, with the e key that changes it.
func (m model) closedRow(p pal, a store.Application, w int, keep func(string) bool) []string {
	if a.CloseReason == "" || !endedStatus(a.Status) {
		return nil
	}
	key, action := "e", descEdit
	if !keep(key) {
		key, action = "", ""
	}
	return []string{p.cardRow("CLOSED", p.Text(p.Fg).Render(closeReasonLabel(a.CloseReason)), key, action, w)}
}

// endedStatus is a Status that carries a close_reason: Rejected or Withdrawn.
func endedStatus(status string) bool {
	return status == store.StatusRejected || status == store.StatusWithdrawn
}
