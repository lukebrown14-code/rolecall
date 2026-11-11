package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// Fit sits beside JD: the two are read together.
const (
	tabJD = iota
	tabFit
	tabNotes
	tabDocs
)

var detailTabs = []string{"JD", "Fit", "Notes", "Docs"}

// markdownTabFiles are the files shown by the Markdown tabs, indexed by tab.
var markdownTabFiles = map[int]string{tabJD: "jd.md", tabNotes: "notes.md", tabFit: "fit.md"}

// detailState is the open detail view. It is replaced, never mutated in place, so
// copies of the model stay independent.
type detailState struct {
	folder string
	tab    int
	docs   int // selected entry on the Docs tab
	body   viewport.Model
}

// docEntry is one line of the Docs tab.
type docEntry struct {
	name    string // path relative to the Application folder
	present bool
}

func (e docEntry) markdown() bool { return strings.HasSuffix(e.name, ".md") }

// group is the Docs tab section an entry belongs to.
func (e docEntry) group() string {
	switch {
	case strings.HasPrefix(e.name, "out/"):
		return "PDFs · out/"
	case strings.HasPrefix(e.name, "sent/"):
		return "Sent"
	}
	return "Source"
}
func (e docEntry) pdf() bool { return strings.HasSuffix(e.name, ".pdf") }

func (e docEntry) label() string {
	switch {
	case e.markdown() && e.present:
		return e.name + " (present)"
	case e.markdown():
		return e.name + " (absent)"
	}
	return e.name
}

func docEntries(a store.Application) []docEntry {
	entries := []docEntry{{"resume.md", a.HasResume}, {"cover-letter.md", a.HasCoverLetter}, {"gaps.md", a.HasGaps}}
	for _, pdf := range a.PDFs {
		entries = append(entries, docEntry{"out/" + pdf, true})
	}
	if a.HasSent {
		entries = append(entries, docEntry{"sent/", true})
	}
	return entries
}

func (m model) application(folder string) (store.Application, bool) {
	for _, a := range m.apps {
		if a.Folder == folder {
			return a, true
		}
	}
	return store.Application{}, false
}

// current is the Application that row actions apply to: the open detail view's
// Application, otherwise the table's selected row.
func (m model) current() (store.Application, bool) {
	if m.detail != nil {
		return m.application(m.detail.folder)
	}
	rows := m.rows()
	if len(rows) == 0 {
		return store.Application{}, false
	}
	return rows[m.cursor], true
}

func (m *model) openDetail(folder string) {
	m.detail = &detailState{folder: folder, body: viewport.New()}
}

func (m model) closeDetail() model {
	if m.detail == nil {
		return m
	}
	for i, r := range m.rows() {
		if r.Folder == m.detail.folder {
			m.cursor = i
		}
	}
	m.detail = nil
	return m
}

// Detail view sizing.
const (
	cardWidth     = 34 // the left card, from splitWidth up
	minBodyHeight = 3
	detailChrome  = 10 // top bar, gap, title, facts, rule, tabs, gap … cue, rule, keys
	detailFrame   = 7  // the lines around the panes: top bar, gap, title, facts, rule … rule, keys
	maxReadWidth  = 80 // long text wraps at or before this
	minWrapWidth  = 20
	paneMargin    = 4
)

// cardShown reports whether the detail view has room for the left card.
func (m model) cardShown() bool { return m.viewWidth() >= splitWidth }

// paneWidth is the right-hand pane: everything beside the card.
func (m model) paneWidth() int {
	if m.cardShown() {
		return m.viewWidth() - cardWidth - 1
	}
	return m.viewWidth()
}

// bodyHeight is the lines of tab content shown at once.
func (m model) bodyHeight() int { return max(minBodyHeight, m.viewHeight()-detailChrome) }

// bodyWidth is the wrap width of tab content: capped for comfortable reading.
func (m model) bodyWidth() int { return max(minWrapWidth, min(maxReadWidth, m.paneWidth()-paneMargin)) }

// syncDetail re-reads the current tab's file, keeping the scroll position, so the
// view follows changes to the file.
func (m model) syncDetail() model {
	if m.detail == nil {
		return m
	}
	a, ok := m.application(m.detail.folder)
	if !ok {
		m.detail = nil
		return m
	}
	d := *m.detail
	d.docs = min(d.docs, len(docEntries(a))-1)
	d.body.SetWidth(m.bodyWidth())
	d.body.SetHeight(m.bodyHeight())
	if name, ok := markdownTabFiles[d.tab]; ok && a.Malformed == nil {
		offset := d.body.YOffset()
		d.body.SetContent(m.markdownBody(a, d.tab, name))
		d.body.SetYOffset(offset)
	}
	m.detail = &d
	return m
}

// mdCache keeps one Glamour renderer per wrap width: building one is slow, and the
// detail view re-renders on every key. Shared by copies of the model.
type mdCache struct {
	width int
	r     *glamour.TermRenderer
}

func (c *mdCache) renderer(width int) (*glamour.TermRenderer, error) {
	if c != nil && c.r != nil && c.width == width {
		return c.r, nil
	}
	r, err := glamour.NewTermRenderer(glamour.WithStyles(markdownStyle()), glamour.WithWordWrap(width))
	if err == nil && c != nil {
		c.width, c.r = width, r
	}
	return r, err
}

// markdownBody renders one of the Markdown tabs, or its empty state.
func (m model) markdownBody(a store.Application, tab int, name string) string {
	var out string
	if tab == tabFit {
		fit := a.Fit
		if fit == "" {
			fit = fitNone
		}
		out = "Fit: " + fit + "\n\n"
	}
	b, err := m.store.ReadDocument(a.Folder, name)
	if err != nil || strings.TrimSpace(string(b)) == "" {
		return out + "No content in " + name
	}
	r, err := m.md.renderer(m.bodyWidth())
	if err == nil {
		var rendered string
		if rendered, err = r.Render(string(b)); err == nil {
			return out + strings.Trim(rendered, "\n")
		}
	}
	return out + string(b)
}

// detailKey handles keys specific to the detail view. Keys it does not handle fall
// through to the shared actions (m, n, u, r, quit); every other key is consumed so
// the table underneath is not driven by accident.
func (m model) detailKey(k tea.KeyPressMsg) (model, tea.Cmd, bool) {
	a, ok := m.application(m.detail.folder)
	if !ok {
		return m.closeDetail(), nil, true
	}
	d := *m.detail
	m.detail = &d
	key := k.String()
	switch key {
	case "esc":
		return m.closeDetail(), nil, true
	case "right", "left":
		step := 1
		if key == "left" {
			step = len(detailTabs) - 1
		}
		d.tab = (d.tab + step) % len(detailTabs)
		d.body.GotoTop()
		return m, nil, true
	case "m", "n", "d", "u", "r", "b", "?", "q", "ctrl+c":
		return m, nil, false
	}
	if a.Malformed != nil {
		return m, nil, true
	}
	if d.tab == tabDocs {
		return m.docsKey(a, &d, key)
	}
	switch key {
	case "down", "j":
		d.body.ScrollDown(1)
	case "up", "k":
		d.body.ScrollUp(1)
	case "pgdown", "space", "f":
		d.body.PageDown()
	case "pgup":
		d.body.PageUp()
	case "g":
		d.body.GotoTop()
	case "G":
		d.body.GotoBottom()
	case "e":
		name := markdownTabFiles[d.tab]
		if d.tab == tabNotes {
			return m, m.editNotes(a.Folder), true
		}
		return m, m.editFile(a.Folder, name), true
	}
	return m, nil, true
}

func (m model) docsKey(a store.Application, d *detailState, key string) (model, tea.Cmd, bool) {
	entries := docEntries(a)
	switch key {
	case "down", "j":
		d.docs = min(d.docs+1, len(entries)-1)
	case "up", "k":
		d.docs = max(d.docs-1, 0)
	case "g":
		d.docs = 0
	case "G":
		d.docs = len(entries) - 1
	case "enter", "e":
		e := entries[d.docs]
		switch {
		case e.pdf() && key == "enter":
			return m, m.runExternal(a.Folder, "open", filepath.Join(a.Folder, e.name)), true
		case e.markdown() && e.present:
			return m, m.editFile(a.Folder, e.name), true
		case e.markdown():
			m.err = fmt.Errorf("%s hasn't been written yet", e.name)
		}
	}
	return m, nil, true
}

// detailView keeps the preview's card on the left and gives the right side to the
// tabs: "open" reads as the same card with room to read. Below splitWidth the card
// is hidden and the tabs take the full width.
func (m model) detailView(p pal) string {
	a, ok := m.application(m.detail.folder)
	if !ok {
		return ""
	}
	w, paneW := m.viewWidth(), m.paneWidth()
	h := max(minBodyHeight, m.viewHeight()-detailFrame)
	title := "  " + p.text(p.dim).Render("‹ esc  ") + p.text(p.dim).Render(a.Company+" · ") + p.text(p.fg).Bold(true).Render(a.Title)
	when := ""
	switch r, days := ago(a.Closes, m.now()); {
	case a.Malformed != nil:
		title = "  " + p.text(p.dim).Render("‹ esc  ") + p.text(p.bad).Bold(true).Render("⚠ "+filepath.Join(a.Folder, "meta.yml"))
	case a.Closes != "" && beforeApplied(tableOf(a)):
		c := p.dim
		if days == 0 {
			c = p.bad
		} else if days > 0 && days <= soonDays {
			c = p.warn
		}
		when = p.text(c).Render("closes " + r)
	case a.AppliedDate != nil:
		when = p.text(p.dim).Render("applied " + m.agoEntry(a.AppliedDate))
	}
	title = spread(xansi.Truncate(title, w-xansi.StringWidth(when)-ellipsis, "…"), when, w-rightMargin)

	// The facts line: Status · Track · closes · applied, with exact dates.
	closes, applied := a.Closes, entryDate(a.AppliedDate)
	if closes == "" {
		closes = "—"
	}
	if applied == "" {
		applied = "—"
	}
	facts := "  " + p.text(p.statusColor(a.Status)).Bold(true).Render(a.Status) + p.text(p.dim).Render(" · "+a.Track+" · closes "+closes+" · applied "+applied)
	var pane string
	if a.Malformed != nil {
		facts = ""
		path := filepath.Join(a.Folder, "meta.yml")
		// The error repeats the path; the line above already shows it in full.
		msg := strings.Replace(a.Malformed.Error(), path, "meta.yml", 1)
		pane = p.text(p.fg).Render(path) + "\n\n" + p.text(p.bad).Render(msg)
	} else {
		pane = m.detailTabsRow(p) + "\n\n" + m.detailContent(p, a, paneW)
	}
	pane = lipgloss.NewStyle().Width(paneW).Height(h).MaxHeight(h).PaddingLeft(gutter).Render(pane)
	body, ruleTop, ruleBot := pane, strings.Repeat("─", w), strings.Repeat("─", w)
	if m.cardShown() {
		sep := p.text(p.faint).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
		card := lipgloss.NewStyle().Width(cardWidth).Height(h).MaxHeight(h).Render(m.detailCard(p, a))
		body = lipgloss.JoinHorizontal(lipgloss.Top, card, sep, pane)
		ruleTop = strings.Repeat("─", cardWidth) + "┬" + strings.Repeat("─", paneW)
		ruleBot = strings.Repeat("─", cardWidth) + "┴" + strings.Repeat("─", paneW)
	}
	return m.topBar(p) + "\n\n" + title + "\n" + facts + "\n" + p.text(p.faint).Render(ruleTop) + "\n" + body + "\n" +
		m.feedbackLine(p, ruleBot) + "\n" + m.keyLine(p, w)
}

func (m model) detailTabsRow(p pal) string {
	tabs := make([]string, len(detailTabs))
	for i, t := range detailTabs {
		tabs[i] = p.text(p.dim).Render(" " + t + " ")
		if i == m.detail.tab {
			tabs[i] = lipgloss.NewStyle().Background(p.accent).Foreground(p.onFill).Bold(true).Render(" " + t + " ")
		}
	}
	return strings.Join(tabs, " ")
}

// detailContent is the current tab: rendered Markdown with a scroll cue, or the
// document list grouped Source / Built / Sent.
func (m model) detailContent(p pal, a store.Application, paneW int) string {
	if m.detail.tab != tabDocs {
		cue := ""
		if b := m.detail.body; b.TotalLineCount() > b.Height() {
			cue = fmt.Sprintf("── %d%% ──", int(b.ScrollPercent()*percent))
			if !b.AtBottom() {
				cue = "↓ more  " + cue
			}
		}
		return m.detail.body.View() + "\n" + strings.Repeat(" ", max(0, paneW-xansi.StringWidth(cue)-paneMargin)) + p.text(p.dim).Render(cue)
	}
	var lines []string
	group := ""
	for i, e := range docEntries(a) {
		if g := e.group(); g != group {
			if group != "" {
				lines = append(lines, "")
			}
			group = g
			lines = append(lines, p.heading(g))
		}
		mark, nameC := p.text(p.ok).Render("✓ "), p.fg
		name := e.name
		if !e.present {
			mark, nameC, name = p.text(p.dim).Render("○ "), p.dim, e.name+"  not written"
		}
		row := "  " + mark + p.text(nameC).Render(name)
		if i == m.detail.docs {
			row = lipgloss.NewStyle().Background(p.sel).Width(paneW - paneMargin).Render(
				p.text(p.accent).Background(p.sel).Render("▌ ") + p.text(p.accent).Background(p.sel).Bold(true).Render(xansi.Strip(mark)+name))
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// detailCard is the left column: status and Fit, documents, timeline, link.
func (m model) detailCard(p pal, a store.Application) string {
	if a.Malformed != nil {
		return ""
	}
	var b strings.Builder
	label, c := p.docsLabel(a)
	b.WriteString(p.statusChip(a.Status) + p.fitText(a) + "\n\n")
	b.WriteString(p.heading("Docs") + "  " + p.text(c).Render(strings.TrimLeft(label, "●○ ")) + "\n")
	for _, e := range docEntries(a) {
		if e.present {
			b.WriteString(p.text(p.ok).Render("✓ ") + p.text(p.fg).Render(e.name) + "\n")
		} else {
			b.WriteString(p.text(p.dim).Render("○ "+e.name) + "\n")
		}
	}
	b.WriteString("\n" + p.heading("Timeline") + "\n" + m.timeline(p, a, false))
	if a.URL != "" {
		b.WriteString("\n" + p.heading("Link") + "\n" + p.text(p.info).Render(xansi.Truncate(a.URL, cardWidth-ellipsis, "…")))
	}
	return lipgloss.NewStyle().Width(cardWidth).Padding(0, 1, 0, gutter).Render(strings.TrimSuffix(b.String(), "\n"))
}
