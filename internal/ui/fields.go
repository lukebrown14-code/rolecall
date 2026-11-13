package ui

import (
	"strings"
	"time"

	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// A field row is "LABEL  value        [key] action": the label muted, the value plain, and
// the key in the Keys colour at the right edge. Detail and Profile draw every field
// this way, so a row says how to change itself.
const (
	fieldLabelW = 8 // the label column, including the gap before the value
	fieldMinVal = 3 // the least value kept before the action words are dropped
)

// notAssessed is the Fit value of an Application with no Fit yet; the list uses it too.
const notAssessed = "·"

// readOnly stands where a row has no key.
const readOnly = "read-only"

// keyChip is "[m]" in the Keys colour.
func (p pal) keyChip(key string) string { return p.Key().Render("[" + key + "]") }

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
			return xansi.StringWidth("[" + key + "] " + action)
		}
		return xansi.StringWidth("[" + key + "]")
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

// fitValue is the Fit as the list words it: its value, or "·" when not assessed.
func (p pal) fitValue(a store.Application) string {
	if a.Fit == "" {
		return p.Text(p.Dim).Render(notAssessed)
	}
	return p.Text(p.FitColor(a.Fit)).Render(a.Fit)
}

// needsPrepare reports whether a missing Document is one prepare-documents writes, which
// only a Lead or Draft is offered, and only once a Harness is set (the key is not listed
// before).
func (m model) needsPrepare(a store.Application, c docChip) bool {
	return !c.ready && c.name != docPDF && m.harness != "" && handoffAllowed(a, skillPrepareDocuments)
}

// needsBuild reports whether a missing PDF can be built: a Lead or Draft, with a Plugin set.
func (m model) needsBuild(a store.Application, c docChip) bool {
	return !c.ready && c.name == docPDF && m.plugin != "" && a.Malformed == nil &&
		(a.Status == store.StatusLead || a.Status == store.StatusDraft)
}

// docRow is one Document: ● ready or ○ missing, with the key that fixes a missing one.
func (m model) docRow(p pal, a store.Application, c docChip, w int) string {
	key, action := "", ""
	switch {
	case c.ready, m.isWriting(a, c.file):
	case m.needsBuild(a, c):
		key, action = "b", "build"
	case m.needsPrepare(a, c):
		key, action = "p", "prepare docs"
	}
	return p.fieldRow("", m.chipText(p, a, c), key, action, w)
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

// timelineName is the Status name's column in the vertical timeline.
const timelineName = 14

// verticalTimeline is the Status log, newest first, one line per entry joined by a │
// connector under each glyph. An empty log says how to start it.
func (m model) verticalTimeline(p pal, a store.Application, w int) []string {
	if len(a.StatusLog) == 0 {
		return []string{p.Text(p.Dim).Render("Not moved yet · ") + p.keyChip("m") + p.Text(p.Dim).Render(" move")}
	}
	var out []string
	for i := len(a.StatusLog) - 1; i >= 0; i-- {
		e := a.StatusLog[i]
		name := p.Text(p.StatusColor(e.Status)).Bold(i == len(a.StatusLog)-1).Render(padRight(e.Status, timelineName))
		line := p.StatusMark(e.Status) + " " + name + m.logDate(p, e)
		out = append(out, xansi.Truncate(line, w, "…"))
		if i > 0 {
			out = append(out, p.Text(p.Rule).Render("│"))
		}
	}
	return out
}

// fieldBlock is an Application's detail as rows inside w cells: Status, Fit, Track,
// close date and Notes, the next step, the Documents with the keys that fix them, and the
// timeline. The pane, the full view's card and the Overview tab all draw it.
func (m model) fieldBlock(p pal, a store.Application, w int) []string {
	notes := p.Text(p.Dim).Render("—")
	if t := m.docText(a, notesFile); t != "" {
		notes = p.Text(p.Fg).Render(markdownMarks.Replace(t))
	}
	fitKey, fitAction := "", readOnly // without a Harness there is nothing to assess it with
	if m.harness != "" {
		fitKey, fitAction = "f", "assess"
		if a.Fit == "" {
			fitAction = "assess fit"
		}
	}
	rows := []string{
		p.fieldRow("STATUS", p.StatusChip(a.Status), "m", "move", w),
		p.fieldRow("FIT", p.fitValue(a), fitKey, fitAction, w),
		p.fieldRow("TRACK", p.Text(p.Fg).Render(a.Track), "e", "edit", w),
	}
	if text, s := closeInfo(a.Closes, m.now()); s != closeNone && beforeApplied(tableOf(a)) {
		rows = append(rows, p.fieldRow("CLOSES", p.CloseStyle(s, p.Text, p.Dim).Render(text), "e", "edit", w))
	}
	rows = append(rows, p.fieldRow("NOTES", notes, "n", "notes", w))
	if next := m.nextLine(p, a, w); next != "" {
		rows = append(rows, "", next)
	}
	rows = append(rows, "", p.fieldRow("DOCS", p.DocsMeter(a), "", "", w))
	for _, c := range docChips(a) {
		rows = append(rows, m.docRow(p, a, c, w))
	}
	if a.ResumeOnly && !a.CoverLetterHasText {
		rows = append(rows, p.Text(p.Dim).Render("  "+docCoverLetter), p.Text(p.Dim).Render("  "+resumeOnlyNote))
	}
	rows = append(rows, "", p.Text(p.Dim).Render("TIMELINE"))
	return append(rows, m.verticalTimeline(p, a, w)...)
}
