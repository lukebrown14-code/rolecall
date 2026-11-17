package ui

import (
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

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

// contactRows are the card's CONTACTS rows: one line per Contact, a long one wrapped
// under itself.
func (m model) contactRows(p pal, a store.Application, w int) []string {
	var rows []string
	indent := strings.Repeat(" ", cardLabelW)
	for i, c := range a.Contacts {
		label := ""
		if i == 0 {
			label = "CONTACTS"
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

// nextStepRow is the card's NEXT STEP row, with the e key that edits it.
func (m model) nextStepRow(p pal, a store.Application, w int, keep func(string) bool) []string {
	text, s := m.nextStepInfo(a.NextStep)
	if s == closeNone || !openStatus(a.Status) {
		return nil
	}
	key, action := "e", descEdit
	if !keep(key) {
		key, action = "", ""
	}
	return []string{p.cardRow("NEXT STEP", p.CloseStyle(s, p.Text, p.Dim).Render(text), key, action, w)}
}

// closedRow is the card's CLOSED row: why a Rejected or Withdrawn Application ended.
func (m model) closedRow(p pal, a store.Application, w int) []string {
	if a.CloseReason == "" || !(a.Status == store.StatusRejected || a.Status == store.StatusWithdrawn) {
		return nil
	}
	return []string{p.cardRow("CLOSED", p.Text(p.Fg).Render(a.CloseReason), "", "", w)}
}
