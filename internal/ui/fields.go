package ui

import (
	"slices"
	"strings"
	"time"

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

// notNeededMark marks a Document the Application does not need (a Resume only Cover letter).
// The em dash is the frame's "none", as an empty NOTES row shows it.
const notNeededMark = "—"

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
// only a Lead or Draft is offered. The key shows before a Harness is set too: pressing it
// then names the setting that turns it on.
func (m model) needsPrepare(a store.Application, c docChip) bool {
	return !c.ready && c.name != docPDF && m.handoffAllowed(a, skillPrepareDocuments)
}

// needsBuild reports whether a missing PDF can be built: a Lead or Draft. Without a Plugin
// set, b names the setting.
func (m model) needsBuild(a store.Application, c docChip) bool {
	return !c.ready && c.name == docPDF && a.Malformed == nil && a.Unsent()
}

// logDate is a Status entry's date as the timeline words it, "~" for an approximate one.
func (m model) logDate(p pal, e store.StatusEntry) string {
	if m.futureDate(&e) {
		return p.Text(p.Warn).Render(futureFlag)
	}
	t, err := time.Parse("2006-01-02", strings.Split(e.At, "T")[0])
	if err != nil {
		return ""
	}
	d := shortDate(t, m.now())
	if e.Approx {
		d = "~" + d
	}
	return p.Text(p.Dim).Render(d)
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
		if a.Status != "" {
			stop(a.Status, p.Text(p.Dim).Render("not logged"), true)
		}
	}
	for i, e := range a.StatusLog {
		stop(e.Status, m.logDate(p, e), i == len(a.StatusLog)-1)
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
// timeline and the Notes. Each section offers at most one key. The pane, the full view's
// card and the Overview tab all draw it.
func (m model) fieldBlock(p pal, a store.Application, w int) []string {
	rows := m.statusLine(p, a, w)
	if text, s := closeInfo(a.Closes, m.now()); s != closeNone && beforeApplied(tableOf(a)) {
		rows = append(rows, p.fieldRow("CLOSES", p.CloseStyle(s, p.Text, p.Dim).Render(text), "e", "edit", w))
	}
	rows = append(rows, "")
	rows = append(rows, m.docsBlock(p, a, w)...)
	if next := m.nextLine(p, a, w, true); next != "" {
		rows = append(rows, "", next)
	}
	rows = append(rows, "", p.Text(p.Dim).Render("TIMELINE"))
	rows = append(rows, m.verticalTimeline(p, a, w)...)
	return append(rows, "", m.notesRow(p, a, w))
}

// statusLine is "● Applied" with the Fit at the right edge: "fit not assessed" until one is.
// Where both do not fit on one line, the Fit takes the line below.
func (m model) statusLine(p pal, a store.Application, w int) []string {
	fit := p.Text(p.Dim).Render("fit not assessed")
	if a.Fit != "" {
		fit = p.Text(p.Dim).Render("fit ") + p.Text(p.FitColor(a.Fit)).Render(a.Fit)
	}
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
