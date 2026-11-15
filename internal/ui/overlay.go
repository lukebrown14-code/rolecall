package ui

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
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
	b.WriteString(p.Text(p.Fg).Bold(true).Render(xansi.Truncate(titleLabel(a.Title), cw, "…")) + "\n")
	b.WriteString(p.Text(p.Dim).Render(xansi.Truncate(a.Company, cw-statusColumn-gutter, "…")+" · now ") + p.StatusChip(a.Status) + "\n\n")
	likely := likelyCount[a.Status]
	for i, s := range mv.options {
		if i == likely {
			b.WriteString(p.Text(p.Dim).Render(otherStatuses+" ") + p.Text(p.Rule).Render(strings.Repeat("─", cw-len(otherStatuses)-1)) + "\n")
		}
		hint := ""
		if s == store.StatusApplied && a.Documents != store.DocsReady {
			// Allowed, but not silently: the move asks first (requestMove).
			hint = "⚠ not Ready, asks first"
		}
		if a.Status == store.StatusDismissed && i == 0 {
			hint = "restore"
		}
		nameC := p.Fg
		if i >= likely {
			nameC = p.Sub
		}
		row := "  " + p.Key().Render(statusLetters[s]) + "  " + p.StatusMark(s) + " " + p.Text(nameC).Render(padRight(s, statusColumn)) + p.Text(p.Warn).Render(hint)
		if i == mv.sel {
			bg := p.SelBase(true)
			row = p.SelLine(true, cw+gutter, p.SelGutter(true)+
				p.Key().Background(p.Sel).Render(statusLetters[s])+bg.Render("  ")+
				bg.Foreground(p.StatusColor(s)).Render(p.StatusGlyph(s))+
				p.SelText(true).Render(" "+padRight(s, statusColumn))+
				bg.Foreground(p.Warn).Render(hint))
		}
		b.WriteString(row + "\n")
	}
	return popup(p, "Move", false, popSmall, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"),
		p.Keys(mv.Keys()))
}

func (m model) confirmBox(p pal, cm *confirmMode) string {
	a := cm.app
	n, total := readyCount(a)
	// Name what is missing, as that is what the Owner weighs before y.
	var missing []string
	for _, c := range docChips(a) {
		if !c.ready {
			missing = append(missing, c.name)
		}
	}
	state := fmt.Sprintf("%d of %d documents ready", n, total)
	if len(missing) > 0 {
		state += ". Missing: " + strings.Join(missing, ", ")
	}
	name := appName(a.Title)
	if c := strings.TrimSpace(a.Company); c != "" {
		name += " · " + c
	}
	body := lipgloss.NewStyle().Foreground(p.Fg).Width(popSmall-popInset).Render(lipgloss.NewStyle().Bold(true).Render(name)+": "+state+".") +
		"\n\n" + p.Text(p.Sub).Render("Mark it Applied anyway?")
	return popup(p, "Docs not Ready", true, popSmall, m.viewWidth(), body, p.Keys(cm.Keys()))
}

func (m model) overwriteBox(p pal, dm *downloadMode) string {
	d := dm.state
	body := p.Text(p.Fg).Bold(true).Render(d.clash()) + p.Text(p.Sub).Render(" is already in "+displayPath(m.downloads)+".") + "\n" +
		p.Text(p.Dim).Render(fmt.Sprintf("File %d of %d", d.next+1, len(d.app.PDFs)))
	return popup(p, "File exists", true, popSmall, m.viewWidth(), body,
		p.Keys(dm.Keys()))
}

// buildFailBox shows a failed build's own output, until esc.
func (m model) buildFailBox(p pal, bf *buildFailMode) string {
	body := p.Text(p.Fg).Bold(true).Render(bf.failure.title) + "\n\n" + p.Text(p.Sub).Render(bf.failure.output)
	return popup(p, "Build failed", true, popLarge, m.viewWidth(), body, p.Keys(bf.Keys()))
}

func (m model) duplicateBox(p pal, dm *duplicateMode) string {
	d := dm.existing
	body := p.Text(p.Fg).Bold(true).Render(titleLabel(d.Title)) + p.Text(p.Dim).Render(companySuffix(d.Company)) + "\n" +
		p.Text(p.Sub).Render("is already saved with this URL (") + p.StatusChip(d.Status) + p.Text(p.Sub).Render(").")
	if dm.others > 0 {
		body += "\n" + p.Text(p.Dim).Render(fmt.Sprintf("+%d more with the same URL", dm.others))
	}
	return popup(p, "Duplicate URL", true, popForm, m.viewWidth(), body,
		p.Keys(dm.Keys()))
}
