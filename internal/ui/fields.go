package ui

import (
	"slices"
	"strings"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A field row is "LABEL  value        key action": the label muted, the value plain, and
// the key in the Keys colour at the right edge. Detail and Profile draw every field
// this way, so a row says how to change itself.
const (
	fieldLabelW = 8 // the label column, including the gap before the value
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
	return m.harness != "" && a.JDHasText && !c.ready && c.name != docPDF && m.handoffAllowed(a, skillPrepareDocuments)
}

// needsBuild reports whether a missing PDF can be built now: b works (buildable), with a
// Plugin set.
func (m model) needsBuild(a store.Application, c docChip) bool {
	return m.plugin != "" && !c.ready && c.name == docPDF && buildable(a)
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
	if len(a.StatusLog) == 0 {
		// Nothing logged yet: say so, and how a first move logs it.
		line(p.Text(p.Dim).Render("Not moved yet · ")+p.Key().Render("m")+p.Text(p.Dim).Render(" to move"), "")
	}
	log := effectiveLog(a) // an undone move leaves no stop
	years := m.spansYears(log)
	for i, e := range log {
		stop(e.Status, m.logDate(p, e, years), i == len(log)-1)
	}
	if next := nextStatus(a.Status); next != "" {
		line(p.Text(p.StatusColor(next)).Faint(true).Render(p.StatusGlyph(next)+" "+next), p.Text(p.Dim).Render("next"))
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

// fieldBlock is an Application's detail as rows inside w cells, in the design's order: the
// Status with the Fit, the close date while it matters, the Documents, the next step, the
// timeline and the Notes. Each section offers at most one key. The preview, the detail
// card and the sheet's Overview tab draw it.
func (m model) fieldBlock(p pal, a store.Application, w int) []string {
	return m.fieldRows(p, a, w, true)
}

// overviewRows is the Overview tab's rows: the fieldBlock, less the next step where the
// full-screen detail already keeps it on the line under the tabs.
func (m model) overviewRows(p pal, a store.Application, w int) []string {
	return m.fieldRows(p, a, w, m.sheetShown())
}

func (m model) fieldRows(p pal, a store.Application, w int, next bool) []string {
	rows := m.statusLine(p, a, w)
	if text, s := closeInfo(a.Closes, m.now()); s != closeNone && beforeApplied(tableOf(a)) {
		rows = append(rows, p.fieldRow("CLOSES", p.CloseStyle(s, p.Text, p.Dim).Render(text), "e", "edit", w))
	}
	rows = append(rows, "")
	rows = append(rows, m.docsBlock(p, a, w)...)
	if line := m.nextLine(p, a, w, true); line != "" && next {
		rows = append(rows, "", line)
	}
	rows = append(rows, "", p.Text(p.Dim).Render("TIMELINE"))
	rows = append(rows, m.verticalTimeline(p, a, w)...)
	return append(rows, "", m.notesRow(p, a, w))
}

// statusLine is "● Applied" with the Fit at the right edge: "fit —" until one is assessed.
// Where both do not fit on one line, the Fit takes the line below.
func (m model) statusLine(p pal, a store.Application, w int) []string {
	fit := p.fitWords(a.Fit)
	status := p.Text(p.StatusColor(a.Status)).Bold(true).Render(p.StatusGlyph(a.Status) + " " + a.Status)
	if xansi.StringWidth(status)+gutter+xansi.StringWidth(fit) > w {
		return []string{xansi.Truncate(status, w, "…"), xansi.Truncate(fit, w, "…")}
	}
	return []string{spread(status, fit, w)}
}

// docsBlock is "DOCS ■■□□ 2/4" with the one key that fixes the next missing Document, then
// the Documents side by side, wrapped where they do not fit on one line.
func (m model) docsBlock(p pal, a store.Application, w int) []string {
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
	docs := make([]string, 0, len(docChips(a))+1)
	for _, c := range docChips(a) {
		docs = append(docs, m.chipText(p, a, c))
	}
	if a.ResumeOnly && !a.CoverLetterHasText {
		// Marked like the other Documents: neither ready nor missing, but not needed.
		docs = append(docs, p.Text(p.Dim).Render(m.glyph(notNeededMark)+" "+docCoverLetter))
	}
	rows := []string{p.fieldRow("DOCS", p.DocsMeter(a), key, action, w)}
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

// notesRow is the Notes' first line, or "none yet" with the key that adds them.
func (m model) notesRow(p pal, a store.Application, w int) string {
	if t := m.docText(a, notesFile); t != "" {
		return p.fieldRow("NOTES", p.Text(p.Fg).Render(markdownMarks.Replace(t)), "n", "notes", w)
	}
	return p.fieldRow("NOTES", p.Text(p.Dim).Render("none yet"), "n", "add", w)
}
