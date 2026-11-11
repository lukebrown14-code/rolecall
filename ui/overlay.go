package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// likelyCount is how many of MoveOptions form the "likely next" group (spec → Move popup).
// Group sizes come from the spec's Move popup table.
var likelyCount = map[string]int{"Lead": 2, "Draft": 1, "Applied": 3, "Interviewing": 3, "Offer": 2, "Rejected": 2, "Withdrawn": 2, "Dismissed": 1} //nolint:mnd // spec table

const statusColumn = 14 // width of a Status name in the move pop-up

// overlayBox is the pop-up open over the current view, or "" when none is.
func (m model) overlayBox(p pal) string {
	switch {
	case m.form != nil:
		return m.formBox(p)
	case m.duplicate != nil:
		return m.duplicateBox(p)
	case len(m.options) > 0:
		return m.moveBox(p)
	case m.confirm != nil:
		return m.confirmBox(p)
	case m.pendingDownload != nil:
		return m.overwriteBox(p)
	case m.full:
		return m.helpBox(p)
	}
	return ""
}

func (m model) moveBox(p pal) string {
	a, _ := m.application(m.moveTarget)
	cw := popSmall - popInset
	var b strings.Builder
	b.WriteString(p.text(p.fg).Bold(true).Render(xansi.Truncate(a.Title, cw, "…")) + "\n")
	b.WriteString(p.text(p.dim).Render(xansi.Truncate(a.Company, cw-statusColumn-gutter, "…")+" · now ") + p.statusChip(a.Status) + "\n\n")
	likely := likelyCount[a.Status]
	for i, s := range m.options {
		if i == likely {
			b.WriteString(p.text(p.dim).Render("other ") + p.text(p.faint).Render(strings.Repeat("─", cw-len("other "))) + "\n")
		}
		hint := ""
		if s == "Applied" && a.Documents != "Ready" {
			hint = "⚠ docs not built"
		}
		if a.Status == "Dismissed" && i == 0 {
			hint = "restore"
		}
		nameC := p.fg
		if i >= likely {
			nameC = p.sub
		}
		row := "  " + p.text(p.accent).Bold(true).Render(statusLetters[s]) + "  " + p.text(nameC).Render(padRight(s, statusColumn)) + p.text(p.warn).Render(hint)
		if i == m.moveSel {
			sel := lipgloss.NewStyle().Background(p.sel)
			row = sel.Width(cw).Render(p.text(p.accent).Background(p.sel).Render("▌ ") +
				p.text(p.accent).Background(p.sel).Bold(true).Render(statusLetters[s]) +
				p.text(p.fg).Background(p.sel).Bold(true).Render("  "+padRight(s, statusColumn)) +
				p.text(p.warn).Background(p.sel).Render(hint))
		}
		b.WriteString(row + "\n")
	}
	return popup(p, "Move", false, popSmall, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"),
		p.keys([][2]string{{"letter", "move"}, {"↑↓ enter", "pick"}, {"esc", "cancel"}}))
}

func (m model) confirmBox(p pal) string {
	a := m.confirm.app
	state := "not written yet"
	if a.Documents == "Written" {
		state = "written but not built"
	}
	body := lipgloss.NewStyle().Foreground(p.fg).Width(popSmall-popInset).Render("Docs for "+lipgloss.NewStyle().Bold(true).Render(a.Title)+" are "+state+".") +
		"\n\n" + p.text(p.sub).Render("Mark it Applied anyway?")
	return popup(p, "Docs not built", true, popSmall, m.viewWidth(), body, p.keys([][2]string{{"y", "mark Applied"}, {"n esc", "cancel"}}))
}

func (m model) overwriteBox(p pal) string {
	d := m.pendingDownload
	body := p.text(p.fg).Bold(true).Render(d.clash()) + p.text(p.sub).Render(" is already in "+displayPath(m.downloads)+".") + "\n" +
		p.text(p.dim).Render(fmt.Sprintf("File %d of %d", d.next+1, len(d.app.PDFs)))
	return popup(p, "File exists", true, popSmall, m.viewWidth(), body,
		p.keys([][2]string{{"y", "overwrite"}, {"a", "all"}, {"n", "skip"}, {"esc", "stop"}}))
}

func (m model) duplicateBox(p pal) string {
	d := m.duplicate
	body := p.text(p.fg).Bold(true).Render(d.Title) + p.text(p.dim).Render(" · "+d.Company) + "\n" +
		p.text(p.sub).Render("is already saved with this URL (") + p.statusChip(d.Status) + p.text(p.sub).Render(").")
	return popup(p, "Duplicate URL", true, popForm, m.viewWidth(), body,
		p.keys([][2]string{{"o", "open that one"}, {"s", "save anyway"}, {"esc", "cancel"}}))
}
