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
	likely := likelyCount[a.Status]
	if mv.bulk != nil {
		// Several marked rows: how many and from where; every option is a likely one.
		title, now := bulkSubject(mv.bulk)
		b.WriteString(p.Text(p.Fg).Bold(true).Render(xansi.Truncate(title, cw, "…")) + "\n")
		b.WriteString(p.Text(p.Dim).Render(xansi.Truncate("now "+now, cw, "…")) + "\n\n")
		likely = len(mv.options)
	} else {
		b.WriteString(p.Text(p.Fg).Bold(true).Render(xansi.Truncate(titleLabel(a.Title), cw, "…")) + "\n")
		b.WriteString(p.Text(p.Dim).Render(xansi.Truncate(a.Company, cw-statusColumn-gutter, "…")+" · now ") + p.StatusChip(a.Status) + "\n\n")
	}
	for i, s := range mv.options {
		if i == likely {
			b.WriteString(p.Text(p.Dim).Render(otherStatuses+" ") + p.Text(p.Rule).Render(strings.Repeat("─", cw-len(otherStatuses)-1)) + "\n")
		}
		hint := ""
		if s == store.StatusApplied && mv.bulk == nil && a.Documents != store.DocsReady {
			// Allowed, but not silently: the move asks first (requestMove).
			hint = "⚠ not Ready, asks first"
		}
		if a.Status == store.StatusDismissed && i == 0 && mv.bulk == nil {
			hint = "restore"
		}
		nameC := p.Fg
		if i >= likely {
			nameC = p.Sub
		}
		row := "  " + p.Key().Render(statusLetters[s]) + "  " + p.StatusMark(s) + " " + p.Text(nameC).Render(padRight(s, statusColumn)) + p.Text(p.Warn).Render(hint)
		if i == mv.sel {
			bg, on := p.SelBase(true), p.OnSel()
			row = p.SelLine(true, cw+gutter, p.SelGutter(true)+
				p.Key().Background(p.Sel).Render(statusLetters[s])+bg.Render("  ")+
				bg.Foreground(on.StatusColor(s)).Render(p.StatusGlyph(s))+
				p.SelText(true).Render(" "+padRight(s, statusColumn))+
				bg.Foreground(on.Warn).Render(hint))
		}
		b.WriteString(row + "\n")
	}
	return popup(p, "Move", false, popSmall, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"),
		p.Keys(mv.Keys()))
}

// bulkBox asks before a bulk move: how many, to where, which are left out and which go to
// Applied without Ready Documents.
func (m model) bulkBox(p pal, bm *bulkMode) string {
	cw := popSmall - popInset
	verb := "Move " + applications(len(bm.apps)) + " to " + bm.status + "?"
	if bm.status == store.StatusDismissed {
		verb = "Dismiss " + applications(len(bm.apps)) + "?"
	}
	var lines []string
	lines = append(lines, p.Text(p.Fg).Bold(true).Render(xansi.Truncate(verb, cw, "…")), "")
	const listed = 5
	for i, a := range bm.apps {
		if i == listed {
			lines = append(lines, p.Text(p.Dim).Render(fmt.Sprintf("+%d more", len(bm.apps)-listed)))
			break
		}
		lines = append(lines, p.StatusMark(a.Status)+" "+p.Text(p.Sub).Render(xansi.Truncate(moveName(a), cw-statusMarkWidth, "…")))
	}
	if bm.skipped > 0 {
		lines = append(lines, "", p.Text(p.Dim).Render(fmt.Sprintf("%d marked left out: already there, malformed, or not a Lead or Draft.", bm.skipped)))
	}
	if bm.notRdy > 0 {
		lines = append(lines, "", p.Text(p.Warn).Render(fmt.Sprintf("⚠ %d without Ready Documents.", bm.notRdy)))
	}
	lines = append(lines, "", p.Text(p.Dim).Render("u undoes them all at once."))
	title := "Move"
	if bm.status == store.StatusDismissed {
		title = "Dismiss"
	}
	return popup(p, title, bm.notRdy > 0, popSmall, m.viewWidth(), strings.Join(lines, "\n"), p.Keys(bm.Keys()))
}

func (m model) confirmBox(p pal, cm *confirmMode) string {
	a := cm.app
	n, total := readyCount(a)
	// Name what is missing, as that is what the Owner weighs before y.
	var missing []string
	for _, c := range docChips(a) {
		if !c.ready {
			missing = append(missing, c.todo())
		}
	}
	state := fmt.Sprintf("%d of %d Document steps finished", n, total)
	if len(missing) > 0 {
		state += ". Still to do: " + strings.Join(missing, ", ")
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
		p.Text(p.Dim).Render(fmt.Sprintf("File %d of %d", d.next+1, len(d.pdfs)))
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
