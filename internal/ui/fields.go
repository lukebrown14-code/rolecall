package ui

import (
	"slices"
	"strconv"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A field row is "LABEL  value        key action": the label muted, the value plain, and
// the key in the Keys colour at the right edge. Detail and Profile draw every field
// this way, so a row says how to change itself.
const (
	fieldLabelW = 8 // Profile's label column, including the gap before the value; the card's is cardLabelW
	fieldMinVal = 3 // the least value kept before the action words are dropped
)

// untitled stands in for a blank Title wherever one is drawn: a row, the preview, a
// header, a pop-up or a message.
const untitled = "(untitled)"

// titleLabel is a Title as a row or header draws it: as stored, or untitled when blank.
func titleLabel(title string) string {
	if strings.TrimSpace(title) == "" {
		return untitled
	}
	return title
}

// companySuffix is " · Company" after a title, or nothing when the company is blank, so
// no separator is left hanging.
func companySuffix(company string) string {
	if strings.TrimSpace(company) == "" {
		return ""
	}
	return " · " + company
}

// notNeededMark marks a Document the Application does not need (a Resume only Cover letter).
// The em dash is the frame's "none", as an empty NOTES row shows it.
const notNeededMark = glyphNone

// keyChip is the bare key, "m", in the Keys colour.
func (p pal) keyChip(key string) string { return p.Key().Render(key) }

// fieldRow draws one row inside w cells. The value truncates first; when even that is not
// enough the action words go, then the key. A row with an action but no key is read-only.
func (p pal) fieldRow(label, value, key, action string, w int) string {
	return p.fieldRowIn(fieldLabelW, label, value, key, action, w)
}

// fieldRowIn is fieldRow with a label column of labelW cells, for a block whose labels are
// longer than the usual column (Profile's Contact).
func (p pal) fieldRowIn(labelW int, label, value, key, action string, w int) string {
	lead := p.Text(p.Dim).Render(padRight(label, labelW))
	keyW := func(withAction bool) int {
		switch {
		case key == "":
			return 0
		case withAction && action != "":
			return xansi.StringWidth(key + " " + action)
		}
		return xansi.StringWidth(key)
	}
	withAction := w-labelW-keyW(true)-1 >= fieldMinVal
	if key != "" && w-labelW-keyW(withAction)-1 < 1 {
		key, action = "", ""
	}
	right := ""
	if key == "" && action != "" {
		right = p.Text(p.Dim).Render(action)
	}
	if key != "" {
		right = p.keyChip(key)
		if withAction && action != "" {
			right += p.Text(p.Dim).Render(" " + action)
		}
	}
	avail := max(0, w-labelW-xansi.StringWidth(right))
	if right != "" {
		avail = max(0, avail-1)
	}
	value = xansi.Truncate(value, avail, "…")
	gap := max(0, w-labelW-xansi.StringWidth(value)-xansi.StringWidth(right))
	if right != "" {
		gap = max(1, gap)
	}
	return lead + value + strings.Repeat(" ", gap) + right
}

// markdownMarks drops emphasis and code marks from a Notes line shown as plain text.
var markdownMarks = strings.NewReplacer("**", "", "*", "", "`", "")

// needsPrepare reports whether a missing Document is one prepare-documents writes, which
// only a Lead or Draft with a JD is offered, and only with a Harness set: a key is offered
// only where it works.
func (m model) needsPrepare(a store.Application, c docChip) bool {
	return m.harness != "" && a.JDHasText && !c.ready && c.step == stepWritten && m.handoffAllowed(a, skillPrepareDocuments)
}

// needsBuild reports whether a missing PDF can be built now: b works (buildable), with a
// Plugin set.
func (m model) needsBuild(a store.Application, c docChip) bool {
	return m.plugin != "" && !c.ready && c.step == stepBuilt && buildable(a)
}

// logDate is a Status entry's date as the timeline words it, "~" for an approximate one.
// With year it always carries the year, "1 Jan 2026", so a log that spans years never
// reads as two stops on one day.
func (m model) logDate(p pal, e store.StatusEntry, year bool) string {
	if m.futureDate(&e) {
		return p.Text(p.Warn).Render(futureFlag)
	}
	t, ok := parseDay(e.At)
	if !ok {
		return ""
	}
	d := shortDate(t, m.now())
	if year {
		d = t.Format(fullDate)
	}
	if e.Approx {
		d = "~" + d
	}
	return p.Text(p.Dim).Render(d)
}

// fullDate is a date in full as the detail words it: "25 Sep 2026".
const fullDate = "2 Jan 2006"

// fullEntryDate is a Status entry's date in full, "25 Sep 2026", or "~1 Jan 2025" when
// approximate: the one date format the detail view uses.
func fullEntryDate(e *store.StatusEntry) string {
	t, ok := parseDay(e.At)
	if !ok {
		return entryDate(e)
	}
	d := t.Format(fullDate)
	if e.Approx {
		d = "~" + d
	}
	return d
}

// spansYears reports whether a Status log has an entry outside this year, so its timeline
// gives every date its year.
func (m model) spansYears(log []store.StatusEntry) bool {
	return slices.ContainsFunc(log, func(e store.StatusEntry) bool {
		t, ok := parseDay(e.At)
		return ok && t.Year() != m.now().Year()
	})
}

// verticalTimeline is the Status log oldest first, one indented line per entry with its
// date at the right edge, the current Status bold. Below it, faint, is the Status the
// pipeline goes to next; an empty log shows the current Status as its one stop.
func (m model) verticalTimeline(p pal, a store.Application, w int) []string {
	var out []string
	line := func(left, right string) {
		left = "  " + left
		if xansi.StringWidth(left)+gutter+xansi.StringWidth(right) > w {
			out = append(out, xansi.Truncate(left, w, "…"))
			return
		}
		out = append(out, spread(left, right, w))
	}
	stop := func(status, when string, current bool) {
		line(p.StatusMark(status)+" "+p.Text(p.StatusColor(status)).Bold(current).Render(status), when)
	}
	log := effectiveLog(a) // an undone move leaves no stop
	if len(log) == 0 {
		// Nothing logged yet (or only an undone first move): say so, and how a first move logs it.
		line(p.Text(p.Dim).Render("Not moved yet · ")+p.Key().Render("m")+p.Text(p.Dim).Render(" to move"), "")
	}
	years := m.spansYears(log)
	for i, e := range log {
		stop(e.Status, m.logDate(p, e, years), i == len(log)-1)
	}
	if next := nextStatus(a.Status); next != "" {
		line(p.Text(p.Faint).Render(p.StatusGlyph(next))+" "+p.Text(p.Dim).Render(next), p.Text(p.Dim).Render("next"))
	}
	return out
}

// nextStatus is the Status after status along the pipeline, "" past Offer or once closed.
func nextStatus(status string) string {
	i := slices.Index(pipeline, status)
	if i < 0 || i+1 >= len(pipeline) {
		return ""
	}
	return pipeline[i+1]
}

// cardMode is where an Application's card rows are drawn, which decides what they leave to
// the frame around them.
type cardMode int

const (
	cardPreview cardMode = iota // beside the list, with the start of fit.md and jd.md below
	cardDetail                  // beside the detail tabs, with the link below
	cardFull                    // the narrow detail's Overview: the strip says the Status, Documents and Fit, the line under the tabs the next step
)

// cardLabelW is the card's one label column: its widest label, SPONSORSHIP, and a
// two-cell gap, so every value in a card starts at the same cell.
const cardLabelW = len("SPONSORSHIP") + 2

// cardRow is a field row in the card's label column.
func (p pal) cardRow(label, value, key, action string, w int) string {
	return p.fieldRowIn(cardLabelW, label, value, key, action, w)
}

// blockLabel is a card block's heading, TIMELINE, FIT or JD: a field label on a line of its
// own, so the card has one heading style.
func (p pal) blockLabel(label string) string { return p.Text(p.Dim).Render(label) }

// fitUnassessed is a Fit not yet assessed, as the card and the Fit tab word it.
const fitUnassessed = "not assessed"

// cardFit is the card's Fit: "fit Good" in the Fit's colour, or "fit not assessed".
func (p pal) cardFit(fit string) string {
	if fit == "" {
		return p.Text(p.Dim).Render(fitLabel + fitUnassessed)
	}
	return p.fitWords(fit)
}

// overviewRows is the Overview tab's rows: the card's, less what the frame around them
// already says.
func (m model) overviewRows(p pal, a store.Application, w int) []string {
	return m.fieldRows(p, a, w, cardFull)
}

// fieldRows is an Application's card as rows inside w cells, in the design's order: the
// Status with its age and the Fit, the next step, the facts (the close date while it
// matters, what the Sweep and the ad recorded), the Documents, the timeline and the
// Notes. A key shows once: a row whose key the next step offers leaves it to NEXT.
func (m model) fieldRows(p pal, a store.Application, w int, mode cardMode) []string {
	return m.cardRows(p, a, w, mode, false)
}

// cardRows is fieldRows, without what the Sweep and the ad recorded (POSTED, TYPE, PAY,
// LISTING…) when lean: a card short of room folds those first, so NEXT, the next step and
// the TIMELINE keep their lines.
func (m model) cardRows(p pal, a store.Application, w int, mode cardMode, lean bool) []string {
	nextKey := ""
	if s, ok := m.nextAction(a); ok {
		nextKey = s.key
	}
	keep := func(key string) bool { return key != nextKey }
	var rows []string
	section := func(lines ...string) {
		if len(lines) == 0 {
			return
		}
		if len(rows) > 0 {
			rows = append(rows, "")
		}
		rows = append(rows, lines...)
	}
	var head []string
	switch mode {
	case cardPreview, cardDetail:
		head = m.statusLine(p, a, w, true)
		if line := m.nextLine(p, a, w, true); line != "" {
			head = append(head, line)
		}
	}
	section(head...)
	facts := m.factRows(p, a, w, keep)
	if !lean {
		facts = append(facts, m.sweepRows(p, a, w)...)
	}
	section(facts...)
	if mode != cardFull {
		section(m.docsBlock(p, a, w, keep)...)
	}
	if line := m.followedUpRow(p, a, w); line != "" {
		section(line)
	}
	section(append([]string{p.blockLabel("TIMELINE")}, m.verticalTimeline(p, a, w)...)...)
	section(m.contactRows(p, a, w)...)
	section(m.notesRow(p, a, w, keep))
	return rows
}

// statusLine is "● Applied · 6w in Applied" with the Fit at the right edge. Without status
// (the sheet, whose title chip says it) it is the age and the Fit. Short of room the age
// drops the Status it repeats ("● Applied · 6w"); where the halves still do not fit on one
// line, the Fit takes the line below.
func (m model) statusLine(p pal, a store.Application, w int, status bool) []string {
	fit := p.cardFit(a.Fit)
	left := func(long bool) string {
		var parts []string
		if status {
			parts = append(parts, p.Text(p.StatusColor(a.Status)).Bold(true).Render(p.StatusGlyph(a.Status)+" "+a.Status))
		}
		if age := m.stageAge(p, a, long || !status); age != "" {
			parts = append(parts, age)
		}
		return strings.Join(parts, p.Text(p.Dim).Render(" · "))
	}
	fits := func(l string) bool { return xansi.StringWidth(l)+gutter+xansi.StringWidth(fit) <= w }
	l := left(true)
	if !fits(l) {
		l = left(false)
	}
	switch {
	case l == "":
		return []string{xansi.Truncate(fit, w, "…")}
	case !fits(l):
		return []string{xansi.Truncate(l, w, "…"), xansi.Truncate(fit, w, "…")}
	}
	return []string{spread(l, fit, w)}
}

// stageAge is how long the Application has had its Status, as the card's Status line says
// it: "added 1d ago" for a Lead never moved, "6w in Applied" while it is open, "today" on
// the day it moved, and "3d ago" once closed or dismissed, with SCHEMA's ~ for an
// approximate date. Not long, an open one leaves off the Status, "6w", for a line that
// already names it. The list rows leave the age to the card.
func (m model) stageAge(p pal, a store.Application, long bool) string {
	e := lastChange(a)
	if e == nil {
		return ""
	}
	if m.futureDate(e) {
		return p.Text(p.Warn).Render(futureFlag)
	}
	_, days, ok := daysFrom(e.At, m.now())
	if !ok {
		return ""
	}
	approx := ""
	if e.Approx {
		approx = "~"
	}
	rel, _ := ago(e.At, m.now())
	var s string
	switch t := tableOf(a); {
	case a.Status == store.StatusLead && len(effectiveLog(a)) <= 1:
		s = "added " + approx + rel
	case t == tableDismissed || t == tableFiltered || t == tableClosed:
		s = approx + rel
	case days == 0:
		s = approx + rel
	default:
		s = approx + span(max(-days, 0))
		if long {
			s += " in " + a.Status
		}
	}
	return p.Text(p.Dim).Render(s)
}

// factRows are the card's facts: the close date while it matters, the next step and the
// close reason. What the Sweep and the ad recorded (sweepRows) follows them on the card.
func (m model) factRows(p pal, a store.Application, w int, keep func(string) bool) []string {
	var rows []string
	key, action := "e", descEdit
	if !keep(key) {
		key, action = "", ""
	}
	if text, s := closeInfo(a.Closes, m.now()); s != closeNone && beforeApplied(tableOf(a)) {
		rows = append(rows, p.cardRow("CLOSES", p.CloseStyle(s, p.Text, p.Dim).Render(text), key, action, w))
	} else if a.ClosesUnclear && a.Closes == "" && beforeApplied(tableOf(a)) {
		rows = append(rows, p.cardRow("CLOSES", p.Text(p.Warn).Render(closesUnclearText), key, action, w))
	}
	rows = append(rows, m.nextStepRow(p, a, w, keep)...)
	return append(rows, m.closedRow(p, a, w, keep)...)
}

// docsBlock is "DOCS ■■□□ 2/4" with the one key that fixes the next missing Document,
// unless NEXT already offers it, then the Documents side by side, wrapped where they do
// not fit on one line.
func (m model) docsBlock(p pal, a store.Application, w int, keep func(string) bool) []string {
	key, action := "", ""
	for _, c := range docChips(a) {
		if c.ready || m.isWriting(a, c.file) {
			continue
		}
		if m.needsPrepare(a, c) {
			key, action = "p", "prepare docs"
			break
		}
		if m.needsBuild(a, c) && key == "" {
			key, action = "b", "build"
		}
	}
	if !keep(key) {
		key, action = "", ""
	}
	docs := m.docMarks(p, a)
	if a.ResumeOnly && !a.CoverLetterHasText {
		// Marked like the other Documents: neither ready nor missing, but not needed.
		docs = append(docs, p.Text(p.Dim).Render(m.glyph(notNeededMark)+" "+docCoverLetter))
	}
	rows := []string{p.cardRow("DOCS", p.DocsMeter(a), key, action, w)}
	const indent, sep = "  ", "   "
	line := indent
	for _, d := range docs {
		switch {
		case line == indent:
			line += d
		case xansi.StringWidth(line+sep+d) <= w:
			line += sep + d
		default:
			rows = append(rows, line)
			line = indent + d
		}
	}
	return append(rows, xansi.Truncate(line, w, "…"))
}

// notesRow is the Notes' first line, or "none yet", with n, which reads "notes"
// everywhere, unless NEXT already offers it.
func (m model) notesRow(p pal, a store.Application, w int, keep func(string) bool) string {
	key, value, action := "n", p.Text(p.Dim).Render("none yet"), descNotes
	if t := m.docText(a, notesFile); t != "" {
		value = p.Text(p.Fg).Render(markdownMarks.Replace(t))
	}
	if !keep(key) {
		key, action = "", ""
	}
	return p.cardRow("NOTES", value, key, action, w)
}

// addedOn is the day the Application was added, its first Status entry, as a short date
// ("2 Oct"), or "" when the log does not say.
func (m model) addedOn(a store.Application) string {
	if len(a.StatusLog) == 0 {
		return ""
	}
	t, ok := parseDay(a.StatusLog[0].At)
	if !ok {
		return ""
	}
	return shortDate(t, m.now())
}

// sweepRows are what the Sweep recorded of where a Lead came from: when it was posted,
// how many had applied, its Source, and the Search rule that dismissed it, if one did.
// Each row shows only when the Application says; a long value wraps under itself. A
// figure the Sweep took on another day says which day, so it never reads as a second,
// different age beside today's.
func (m model) sweepRows(p pal, a store.Application, w int) []string {
	var rows []string
	indent := strings.Repeat(" ", cardLabelW)
	add := func(label, value string, extra ...string) {
		lines := wrapWords(value, max(1, w-cardLabelW))
		rows = append(rows, p.cardRow(label, p.Text(p.Fg).Render(lines[0]), "", "", w))
		for _, l := range lines[1:] {
			rows = append(rows, indent+p.Text(p.Fg).Render(l))
		}
		for _, e := range extra {
			rows = append(rows, indent+e)
		}
	}
	warning := func(s string) string {
		return p.Text(p.Warn).Render(xansi.Truncate(glyphWarn+" "+s, max(1, w-cardLabelW), "…"))
	}
	if a.Posted != "" || a.PostedLabel != "" {
		var parts []string
		if a.Posted != "" {
			parts = append(parts, a.Posted)
			if age := postedAge(a, m.now()); age != "" {
				parts = append(parts, age+" ago")
			}
		}
		if a.PostedLabel != "" {
			// The site's own label is as old as the Lead: it was read when the Lead was added.
			said := "site said \"" + a.PostedLabel + "\""
			if on := m.addedOn(a); on != "" {
				said += " on " + on
			}
			parts = append(parts, said)
		}
		var extra []string
		if warn := ageWarning(a, m.rules, m.now()); warn != "" {
			extra = append(extra, warning(warn))
		}
		add("POSTED", strings.Join(parts, " · "), extra...)
	}
	if a.Applicants != nil {
		add("APPLICANTS", strconv.Itoa(*a.Applicants))
	}
	if a.Source != "" {
		add("SOURCE", a.Source)
	}
	for _, f := range m.adFactRows(a) {
		var extra []string
		if f.warn != "" {
			extra = append(extra, warning(f.warn))
		}
		add(f.label, f.value, extra...)
	}
	if a.Filtered() {
		value := filterReasonLabel(a.FilterReason)
		if a.FilterNote != "" {
			// The note was written when the rule dismissed the Lead: its ages are that day's.
			note := a.FilterNote
			if e := lastChange(a); e != nil {
				if t, ok := parseDay(e.At); ok {
					note = "as of " + shortDate(t, m.now()) + ": " + note
				}
			}
			value += " · " + note
		}
		add("FILTERED", value)
	}
	return rows
}
