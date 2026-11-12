package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// likelyCount is how many of MoveOptions form the "likely next" group (spec → Move popup).
// Group sizes come from the spec's Move popup table.
var likelyCount = map[string]int{store.StatusLead: 2, store.StatusDraft: 1, store.StatusApplied: 3, store.StatusInterviewing: 3, store.StatusOffer: 2, store.StatusRejected: 2, store.StatusWithdrawn: 2, store.StatusDismissed: 1} //nolint:mnd // spec table

const statusColumn = 14 // width of a Status name in the move pop-up

// otherStatuses heads the Move pop-up's second group, after the likely next steps.
const otherStatuses = "Other statuses"

func (m model) moveBox(p pal, mv *moveMode) string {
	a := mv.app
	cw := popSmall - popInset
	var b strings.Builder
	b.WriteString(p.text(p.fg).Bold(true).Render(xansi.Truncate(a.Title, cw, "…")) + "\n")
	b.WriteString(p.text(p.dim).Render(xansi.Truncate(a.Company, cw-statusColumn-gutter, "…")+" · now ") + p.statusChip(a.Status) + "\n\n")
	likely := likelyCount[a.Status]
	for i, s := range mv.options {
		if i == likely {
			b.WriteString(p.text(p.dim).Render(otherStatuses+" ") + p.text(p.rule).Render(strings.Repeat("─", cw-len(otherStatuses)-1)) + "\n")
		}
		hint := ""
		if s == store.StatusApplied && a.Documents != store.DocsReady {
			// Allowed, but not silently: the move asks first (requestMove).
			hint = "⚠ not Ready, asks first"
		}
		if a.Status == store.StatusDismissed && i == 0 {
			hint = "restore"
		}
		nameC := p.fg
		if i >= likely {
			nameC = p.sub
		}
		row := "  " + p.key().Render(statusLetters[s]) + "  " + p.statusMark(s) + " " + p.text(nameC).Render(padRight(s, statusColumn)) + p.text(p.warn).Render(hint)
		if i == mv.sel {
			sel := lipgloss.NewStyle().Background(p.sel)
			row = sel.Width(cw + gutter).Render(p.text(p.accent).Background(p.sel).Render("▌ ") +
				p.key().Background(p.sel).Render(statusLetters[s]) + p.text(p.fg).Background(p.sel).Render("  ") +
				p.text(p.statusColor(s)).Background(p.sel).Render(p.statusGlyph(s)) +
				p.text(p.fg).Background(p.sel).Bold(true).Render(" "+padRight(s, statusColumn)) +
				p.text(p.warn).Background(p.sel).Render(hint))
		}
		b.WriteString(row + "\n")
	}
	return popup(p, "Move", false, popSmall, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"),
		p.keys(mv.Keys()))
}

func (m model) confirmBox(p pal, cm *confirmMode) string {
	a := cm.app
	n, total := readyCount(a)
	state := fmt.Sprintf("%d of %d documents ready", n, total)
	if a.Documents == store.DocsWritten {
		state += "; the PDF is missing until you build"
	}
	body := lipgloss.NewStyle().Foreground(p.fg).Width(popSmall-popInset).Render(lipgloss.NewStyle().Bold(true).Render(appName(a.Title))+": "+state+".") +
		"\n\n" + p.text(p.sub).Render("Mark it Applied anyway?")
	return popup(p, "Docs not Ready", true, popSmall, m.viewWidth(), body, p.keys(cm.Keys()))
}

func (m model) overwriteBox(p pal, dm *downloadMode) string {
	d := dm.state
	body := p.text(p.fg).Bold(true).Render(d.clash()) + p.text(p.sub).Render(" is already in "+displayPath(m.downloads)+".") + "\n" +
		p.text(p.dim).Render(fmt.Sprintf("File %d of %d", d.next+1, len(d.app.PDFs)))
	return popup(p, "File exists", true, popSmall, m.viewWidth(), body,
		p.keys(dm.Keys()))
}

// buildFailBox shows a failed build's own output, until esc.
func (m model) buildFailBox(p pal, bf *buildFailMode) string {
	body := p.text(p.fg).Bold(true).Render(bf.failure.title) + "\n\n" + p.text(p.sub).Render(bf.failure.output)
	return popup(p, "Build failed", true, popLarge, m.viewWidth(), body, p.keys(bf.Keys()))
}

func (m model) duplicateBox(p pal, dm *duplicateMode) string {
	d := dm.existing
	body := p.text(p.fg).Bold(true).Render(d.Title) + p.text(p.dim).Render(" · "+d.Company) + "\n" +
		p.text(p.sub).Render("is already saved with this URL (") + p.statusChip(d.Status) + p.text(p.sub).Render(").")
	return popup(p, "Duplicate URL", true, popForm, m.viewWidth(), body,
		p.keys(dm.Keys()))
}
