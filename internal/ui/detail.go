package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
	appearance "github.com/lukebrown14-code/rolecall/internal/ui/theme"
)

// detailTab identifies a tab independently of its position in the visible list.
type detailTab string

const coverLetterFile = "cover-letter.md"

const (
	tabLetter detailTab = docCoverLetter
	tabGaps   detailTab = "Gaps"
	tabResume detailTab = "Resume"
	tabJD     detailTab = "JD"
	tabFit    detailTab = "Fit"
	tabNotes  detailTab = "Notes"
	tabDocs   detailTab = "Docs"
	// tabOverview holds the same rows as the card, for the sheet and full views that have no card.
	tabOverview detailTab = "Overview"
)

// visibleDetailTabs derives navigation from the Application's written documents.
func visibleDetailTabs(a store.Application) []detailTab {
	tabs := []detailTab{tabJD, tabFit}
	if a.ResumeHasText {
		tabs = append(tabs, tabResume)
	}
	if a.CoverLetterHasText {
		tabs = append(tabs, tabLetter)
	}
	if a.GapsHasText {
		tabs = append(tabs, tabGaps)
	}
	return append(tabs, tabNotes, tabDocs)
}

// detailTabs is the tabs the open detail shows. Where the card is folded away (the sheet and
// the narrow full view) an Overview tab, first, holds the card's rows.
func (m model) detailTabs(a store.Application) []detailTab {
	tabs := visibleDetailTabs(a)
	if !m.cardShown() {
		tabs = append([]detailTab{tabOverview}, tabs...)
	}
	return tabs
}

var markdownTabFiles = map[detailTab]string{tabJD: jdFile, tabNotes: notesFile, tabFit: fitFile, tabResume: resumeFile, tabLetter: coverLetterFile, tabGaps: gapsFile}

// detailState is the open detail view. It is replaced, never mutated in place, so
// copies of the model stay independent.
type detailState struct {
	folder string
	tab    detailTab
	docs   int // selected entry on the Docs tab
	body   viewport.Model
}

// docEntry is one line of the Docs tab.
type docEntry struct {
	name    string // path relative to the Application folder
	hasText bool
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

// title is the entry as the Owner knows it: a document's name, a built PDF's own file
// name, or the copy of what was sent.
func (e docEntry) title() string {
	switch {
	case e.markdown():
		return docName(e.name)
	case e.name == "sent/":
		return "Sent copy"
	}
	return strings.TrimPrefix(e.name, "out/")
}

// resumeOnlyNote is the Cover letter's state on a Resume only Application.
const resumeOnlyNote = notNeeded + " · Resume only"

// notNeeded is the state of a Document the Application does not need.
const notNeeded = "not needed"

// displayStatus is a source document's state, ready or missing (an empty file is
// missing), and whether it is ready. Built and sent files are ready by being there.
func (e docEntry) displayStatus(a store.Application) (string, bool) {
	switch {
	case !e.markdown():
		return "", true
	case e.name == coverLetterFile && a.ResumeOnly && !e.hasText:
		return resumeOnlyNote, false
	case !e.hasText:
		return docMissing, false
	}
	return docReady, true
}

func docEntries(a store.Application) []docEntry {
	entries := []docEntry{{jdFile, a.JDHasText}, {resumeFile, a.ResumeHasText}, {coverLetterFile, a.CoverLetterHasText}, {gapsFile, a.GapsHasText}}
	for _, pdf := range a.PDFs {
		entries = append(entries, docEntry{"out/" + pdf, false})
	}
	if a.HasSent {
		entries = append(entries, docEntry{"sent/", false})
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
	return m.selected()
}

// openDetail opens an Application on its first tab: Overview where the card is folded away,
// otherwise the JD. A malformed one opens on the JD, where its parse error shows.
func (m *model) openDetail(folder string) {
	tab := tabJD
	if a, ok := m.application(folder); ok && a.Malformed == nil {
		tab = m.detailTabs(a)[0]
	}
	m.detail = &detailState{folder: folder, tab: tab, body: viewport.New()}
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
	minBodyHeight = 3
	sheetChrome   = 3  // the sheet's title and tabs rows and the scroll cue
	paneChrome    = 3  // the pane's lines around the tab content: tabs row, gap … scroll cue
	maxReadWidth  = 80 // long text wraps at or before this
	minWrapWidth  = 20
	paneMargin    = 4
	docColumn     = 14 // the Docs tab's name and state columns
	stripLines    = 2  // the strip that stands in for the card: timeline track, then documents
)

// cardShown reports whether the detail view has room for the card: the wide size class.
// Below it the card folds into a strip under the title.
func (m model) cardShown() bool { return m.layout().wide() }

// cardWidth is the card's column, divider excluded: the preview's, so opening the detail
// leaves the card where the preview was and only the left side changes.
func (m model) cardWidth() int { return m.layout().detailW }

// detailContextLines is the lines between the tab bar and the body: the title row, which
// stands where a list's filter row does, then without the card the strip in its place.
func (m model) detailContextLines() int {
	if m.cardShown() {
		return tableContext
	}
	return 1 + stripLines
}

// paneWidth is the tabs' pane: everything beside the card, the list's width.
func (m model) paneWidth() int {
	if m.cardShown() {
		return m.viewWidth() - m.cardWidth() - 1
	}
	return m.viewWidth()
}

// bodyHeight is the lines of tab content shown at once.
func (m model) bodyHeight() int {
	if m.sheetShown() {
		_, detail, _ := m.layout().sheetHeights(tableContext)
		return max(minBodyHeight, detail-sheetChrome)
	}
	return max(minBodyHeight, m.layout().bodyHeight(m.detailContextLines())-paneChrome)
}

// bodyWidth is the wrap width of tab content: capped for comfortable reading.
func (m model) bodyWidth() int {
	if m.sheetShown() {
		// The sheet's text runs from the inset to where the list rows' Fit ends.
		return max(minWrapWidth, min(maxReadWidth, m.viewWidth()-inset-rowPad-rightMargin))
	}
	return max(minWrapWidth, min(maxReadWidth, m.paneWidth()-paneMargin))
}

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
	switch {
	case slices.Contains(m.detailTabs(a), d.tab):
	case d.tab == tabOverview:
		d.tab = tabJD // the window widened: the card beside the tabs holds the rows now
	default:
		m.err = fmt.Errorf("%s is gone", docName(markdownTabFiles[d.tab]))
		d.tab = tabDocs
	}
	d.docs = min(d.docs, len(docEntries(a))-1)
	d.body.SetWidth(m.bodyWidth())
	d.body.SetHeight(m.bodyHeight())
	if d.tab == tabOverview && a.Malformed == nil {
		offset := d.body.YOffset()
		d.body.SetContent(strings.Join(m.overviewRows(m.pal(), a, m.bodyWidth()), "\n"))
		d.body.SetYOffset(offset)
	}
	if name, ok := markdownTabFiles[d.tab]; ok && a.Malformed == nil {
		offset := d.body.YOffset()
		d.body.SetContent(m.markdownBody(a, d.tab, name))
		d.body.SetYOffset(offset)
	}
	m.detail = &d
	return m
}

// mdCache keeps one Glamour renderer per wrap width, and the bodies rendered with it,
// so the detail, preview and Profile views do not re-render a Document on every key.
// Shared by copies of the model.
type mdCache struct {
	width  int
	theme  theme
	r      *glamour.TermRenderer
	gen    int               // the Workspace generation bodies were rendered from
	bodies map[string]string // rendered bodies, keyed by width and Document; dropped when gen changes
}

func (c *mdCache) renderer(width int, t theme) (*glamour.TermRenderer, error) {
	if c != nil && c.r != nil && c.width == width && c.theme == t {
		return c.r, nil
	}
	r, err := glamour.NewTermRenderer(glamour.WithStyles(appearance.MarkdownStyle(appearance.Mode(t))), glamour.WithWordWrap(width),
		// Glamour otherwise moves a table's links to a footnote, which empties the
		// Contact table's Email cell; the value belongs in its row.
		glamour.WithInlineTableLinks(true))
	if err == nil && c != nil {
		c.width, c.theme, c.r = width, t, r
	}
	return r, err
}

// body returns the cached body built for one Document key at the current Workspace
// generation and wrap width, building it with render on a miss. A new generation (the
// file watcher or a rescan) drops the cache, so a changed Document is never served stale.
// The cache is keyed on the Document and the wrap width, so a resize, or a switch between
// a screen at width 0 (the preview) and one at the body width, keeps the other's renders
// instead of discarding them.
func (c *mdCache) body(gen, width int, key string, render func() string) string {
	if c == nil {
		return render()
	}
	if c.gen != gen {
		c.gen, c.bodies = gen, nil
	}
	if c.bodies == nil {
		c.bodies = map[string]string{}
	}
	full := strconv.Itoa(width) + "\x00" + key
	if body, ok := c.bodies[full]; ok {
		return body
	}
	body := render()
	c.bodies[full] = body
	return body
}

// renderText draws Markdown with a prepared renderer; a var so a test can count renders.
var renderText = func(r *glamour.TermRenderer, text string) (string, error) { return r.Render(text) }

// markdownBody renders one of the Markdown tabs, or its empty state, which says what to
// press to fill it.
func (m model) markdownBody(a store.Application, tab detailTab, name string) string {
	key := a.Folder + "\x00" + name
	return m.md.body(m.gen, m.bodyWidth(), key, func() string {
		var out string
		b, err := m.store.ReadDocument(a.Folder, name)
		empty := err != nil || strings.TrimSpace(string(b)) == ""
		if tab == tabFit {
			// One line says the verdict and whether its reasoning is saved, and what fills it.
			out = fitLabel + fitSummary(a, !empty)
			if empty {
				if hint := m.emptyDocument(a, tab, name); hint != "" {
					out += " · " + hint
				}
				return out
			}
			out += "\n\n"
		}
		if empty {
			return out + m.emptyDocument(a, tab, name)
		}
		return out + m.renderMarkdown(string(b), m.bodyWidth())
	})
}

// emptyDocument is a tab's empty state: what is missing and the key that fills it.
func (m model) emptyDocument(a store.Application, tab detailTab, name string) string {
	switch tab {
	case tabJD:
		return "No JD saved · e to open " + name
	case tabFit:
		if m.harness != "" {
			return "f to assess fit"
		}
		return ""
	case tabNotes:
		return "No notes yet · e to write some"
	}
	return "No " + docName(name) + " yet · e to open " + name
}

// fitSummary is the Fit with where it comes from when the two sources disagree: the
// verdict lives in meta.yml and its reasoning beside it, so a verdict without reasoning,
// or reasoning without a verdict, says so rather than reading as two different answers.
func fitSummary(a store.Application, reasoning bool) string {
	switch {
	case a.Fit == "" && reasoning:
		return fitNone + " · reasoning saved below"
	case a.Fit == "":
		return fitNone + " · no reasoning saved"
	case !reasoning:
		return a.Fit + " · no reasoning saved"
	}
	return a.Fit
}

// renderMarkdown draws Markdown at a wrap width, falling back to the source when
// Glamour cannot. The renderer is cached per width, so repeated frames are cheap.
func (m model) renderMarkdown(text string, width int) string {
	r, err := m.md.renderer(width, m.theme)
	if err == nil {
		if rendered, err := renderText(r, text); err == nil {
			return strings.Trim(rendered, "\n")
		}
	}
	return text
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
	case "right", "left", "tab", "shift+tab":
		// ←→ switch tabs as they switch views on a list; tab and shift+tab do too, as the
		// detail has one pane.
		tabs := m.detailTabs(a)
		step := 1
		if key == "left" || key == "shift+tab" {
			step = len(tabs) - 1
		}
		d.tab = tabs[(slices.Index(tabs, d.tab)+step)%len(tabs)]
		d.body.GotoTop()
		return m, nil, true
	case "m", "n", "d", "u", "r", "F", "b", "f", "p", "?", "q", "ctrl+c":
		return m, nil, false
	}
	if isScreenKey(key) {
		return m, nil, false // screenKey switches screen and closes the detail
	}
	if a.Malformed != nil {
		if pe, ok := parseError(a.Malformed); ok && key == "e" {
			return m, m.editAt(pe), true
		}
		return m, nil, true
	}
	if d.tab == tabDocs {
		return m.docsKey(a, &d, key)
	}
	if d.tab == tabOverview && key == "e" {
		return m, nil, false // the shared edit key opens the Application's meta.yml
	}
	switch key {
	case "down", "j":
		d.body.ScrollDown(1)
	case "up", "k":
		d.body.ScrollUp(1)
	case "pgdown":
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
			return m, m.openPath(a.Folder, filepath.Join(a.Folder, e.name), false), true
		case e.markdown() && key == "enter" && e.hasText:
			for tab, name := range markdownTabFiles {
				if name == e.name {
					d.tab = tab
					d.body.GotoTop()
					return m.syncDetail(), nil, true
				}
			}
		case e.markdown():
			// e on any document, and Enter on one not written yet, open it in the Editor:
			// writing a missing document is the normal next step, not an error.
			return m, m.editFile(a.Folder, e.name), true
		}
	}
	return m, nil, true
}

// detailView keeps the preview's card on the left and gives the right side to the
// tabs: "open" reads as the same card with room to read. Below the wide size class the
// card folds into a two-line strip under the title (detailStrip) and the tabs take the
// full width.
func (m model) detailView(p pal) string {
	a, ok := m.application(m.detail.folder)
	if !ok {
		return ""
	}
	w, paneW := m.viewWidth(), m.paneWidth()
	h := m.layout().bodyHeight(m.detailContextLines())
	pad := strings.Repeat(" ", inset)
	// Title first, then Company, as the list reads.
	title := pad + p.Text(p.Dim).Render("‹ esc  ") + p.Text(p.Fg).Bold(true).Render(appName(a.Title)) + p.Text(p.Dim).Render(companySuffix(a.CompanyLabel()))
	when := ""
	switch text, s := closeInfo(a.Closes, m.now()); {
	case a.Malformed != nil:
		title = pad + p.Text(p.Dim).Render("‹ esc  ") + p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a))
	case s != closeNone && beforeApplied(tableOf(a)):
		when = p.CloseStyle(s, p.Text, p.Dim).Render(text)
	case appliedDate(a) != nil:
		if m.futureDate(appliedDate(a)) {
			when = p.Text(p.Warn).Render("applied " + futureFlag)
		} else {
			// The exact applied date shows once: here beside the card, on the strip's
			// timeline track without it.
			exact := ""
			if m.cardShown() {
				exact = fullEntryDate(appliedDate(a)) + " · "
			}
			rel := m.agoEntry(appliedDate(a))
			if _, days, _ := daysFrom(appliedDate(a).At, m.now()); exact != "" && -days >= monthYearAfter {
				// An old date already reads as a month: the exact date says it all.
				exact, rel = strings.TrimSuffix(exact, " · "), ""
			}
			when = p.Text(p.Dim).Render("applied " + exact + rel)
		}
	}
	when = m.withChip(p, when)
	title = spread(xansi.Truncate(title, w-xansi.StringWidth(when)-ellipsis, "…"), when, w-rightMargin)

	// The card beside the tabs says the Status, Track and dates, as the preview did; without
	// the card, the strip stands in for it.
	ctx := []string{title}
	if !m.cardShown() {
		ctx = append([]string{title}, m.detailStrip(p, a, w-inset-rightMargin)...)
	}
	var pane string
	if a.Malformed != nil {
		for i := 1; i < len(ctx); i++ {
			ctx[i] = ""
		}
		// Where and how to fix it, the file's Workspace path, and the key that opens it there.
		pane = p.Text(p.Fg).Render(strings.Join(wrapWords(m.malformedReason(a), paneW-paneMargin), "\n")) + "\n\n" + p.Text(p.Dim).Render(m.rel(filepath.Join(a.Folder, "meta.yml"))) +
			"\n\n" + p.Key().Render("e") + p.Text(p.Dim).Render(" fix in editor")
	} else {
		// The tab names sit one cell in from their brackets; the content lines up with them.
		content := strings.Split(m.detailContent(p, a, paneW), "\n")
		for i, l := range content {
			content[i] = " " + l
		}
		// Without the card, the next step takes the line between the tabs and the text on
		// every tab, so it stays in view however short the terminal; Overview leaves it
		// out of its rows (overviewRows).
		gap := ""
		if !m.cardShown() {
			gap = " " + m.nextLine(p, a, paneW-gutter-1-rightMargin, false)
		}
		pane = m.detailTabsRow(p, a, paneW-gutter) + "\n" + gap + "\n" + strings.Join(content, "\n")
	}
	pane = lipgloss.NewStyle().Width(paneW).Height(h).MaxHeight(h).PaddingLeft(gutter).Render(pane)
	body, split, focus := pane, 0, focusNone
	if m.cardShown() {
		// The card stands where the preview stood, at its width, so opening the detail
		// moves nothing on the right: the list on the left gives way to the tabs.
		cardW := m.cardWidth()
		card := lipgloss.NewStyle().Width(cardW).Height(h).MaxHeight(h).Render(m.detailCard(p, a, h))
		// The tab pane scrolls and takes the keys, so its rule is the accent.
		body, split, focus = lipgloss.JoinHorizontal(lipgloss.Top, pane, p.Divider(h), card), paneW, focusLeft
	}
	return m.frame(p, ctx, body, split, focus, m.statusRow(p, m.keyLine(p, w)))
}

// pipeline is the forward run of Statuses a timeline track looks ahead along.
var pipeline = []string{store.StatusLead, store.StatusDraft, store.StatusApplied, store.StatusInterviewing, store.StatusOffer}

// detailStrip is the card folded into two lines for the medium and narrow size classes:
// the timeline as a track, then documents · Fit · close date · Track. Each line sheds
// detail to fit w rather than wrapping.
func (m model) detailStrip(p pal, a store.Application, w int) []string {
	pad := strings.Repeat(" ", inset)
	if a.Malformed != nil {
		return make([]string, stripLines)
	}
	parts := []string{m.fileMarks(p, a), p.fitWords(a.Fit)}
	if beforeApplied(tableOf(a)) {
		if text, s := closeInfo(a.Closes, m.now()); s != closeNone {
			parts = append(parts, p.CloseStyle(s, p.Text, p.Dim).Render(text))
		}
	}
	parts = append(parts, p.Text(p.Dim).Render(a.Track))
	sep := p.Text(p.Dim).Render("  ·  ")
	for len(parts) > 1 && xansi.StringWidth(strings.Join(parts, sep)) > w {
		parts = parts[:len(parts)-1]
	}
	return []string{pad + m.timelineTrack(p, a, w), pad + xansi.Truncate(strings.Join(parts, sep), w, "…")}
}

// timelineTrack is the Status log as one line, oldest first, then the Statuses still
// ahead: "◌ Lead 24 Sep ─ ● Applied 25 Sep ─ ◆ Interviewing ─ ★ Offer". Each Status
// keeps its own glyph: the current one in its colour and bold, earlier ones faint with
// their dates, and ones not reached faint and undated. To fit w it drops the earlier
// dates, then all but the next Status ahead, then the earlier Statuses.
func (m model) timelineTrack(p pal, a store.Application, w int) string {
	past, pastDated, current, ahead := m.trackStops(p, a)
	join := func(stops ...[]string) string {
		var all []string
		for _, s := range stops {
			all = append(all, s...)
		}
		return strings.Join(all, p.Text(p.Rule).Render(" ─ "))
	}
	next := ahead[:min(1, len(ahead))]
	var more []string
	if len(past) > 0 {
		more = []string{p.Text(p.Dim).Render("…")}
	}
	for _, cand := range []string{
		join(pastDated, []string{current}, ahead),
		join(past, []string{current}, ahead),
		join(past, []string{current}, next),
		join(more, []string{current}, next),
		join([]string{current}, next),
	} {
		if xansi.StringWidth(cand) <= w {
			return cand
		}
	}
	return xansi.Truncate(current, w, "…")
}

// trackStops is a timeline track's stops: earlier Statuses without and with their dates,
// the current one, and the Statuses still ahead.
func (m model) trackStops(p pal, a store.Application) (past, pastDated []string, current string, ahead []string) {
	log := effectiveLog(a) // an undone move leaves no stop
	years := m.spansYears(log)
	date := func(e store.StatusEntry) string {
		if d := m.logDate(p, e, years); d != "" {
			return " " + d
		}
		return ""
	}
	for i, e := range log {
		if i == len(log)-1 {
			current = p.StatusMark(e.Status) + " " + p.Text(p.StatusColor(e.Status)).Bold(true).Render(e.Status) + date(e)
			continue
		}
		name := p.Text(p.Faint).Render(p.StatusGlyph(e.Status)) + " " + p.Text(p.Dim).Render(e.Status)
		past = append(past, name)
		pastDated = append(pastDated, name+date(e))
	}
	if current == "" {
		current = p.StatusMark(a.Status) + " " + p.Text(p.StatusColor(a.Status)).Bold(true).Render(a.Status)
	}
	if i := slices.Index(pipeline, a.Status); i >= 0 {
		for _, s := range pipeline[i+1:] {
			ahead = append(ahead, p.Text(p.Faint).Render(p.StatusGlyph(s))+" "+p.Text(p.Dim).Render(s))
		}
	}
	return past, pastDated, current, ahead
}

func (m model) detailTabsRow(p pal, a store.Application, w int) string {
	visible := m.detailTabs(a)
	tabs := make([]string, len(visible))
	for i, t := range visible {
		tabs[i] = p.Text(p.Dim).Render(" " + string(t) + " ")
		if t == m.detail.tab {
			// Bracketed as well as filled: the active tab is marked without colour.
			tabs[i] = p.SelChip(string(t))
		}
	}
	return fitTabs(p, tabs, active(visible, m.detail.tab), w)
}

// active is the index of tab in tabs, or 0.
func active(tabs []detailTab, tab detailTab) int {
	if i := slices.Index(tabs, tab); i >= 0 {
		return i
	}
	return 0
}

// fitTabs joins tabs into one line of at most w cells. Where they do not all fit, it keeps
// a run around the active tab and marks the hidden ends with ‹ and ›: the row never wraps.
func fitTabs(p pal, tabs []string, at, w int) string {
	if line := strings.Join(tabs, " "); xansi.StringWidth(line) <= w {
		return line
	}
	from, to := at, at+1
	width := func(f, t int) int {
		n := xansi.StringWidth(strings.Join(tabs[f:t], " "))
		if f > 0 {
			n += markWidth
		}
		if t < len(tabs) {
			n += markWidth
		}
		return n
	}
	for grew := true; grew; {
		grew = false
		if to < len(tabs) && width(from, to+1) <= w {
			to, grew = to+1, true
		}
		if from > 0 && width(from-1, to) <= w {
			from, grew = from-1, true
		}
	}
	line := strings.Join(tabs[from:to], " ")
	if from > 0 {
		line = p.Text(p.Dim).Render("‹ ") + line
	}
	if to < len(tabs) {
		line += p.Text(p.Dim).Render(" ›")
	}
	return line
}

// detailContent is the current tab: rendered Markdown with a scroll cue, or the
// document list grouped Source / Built / Sent.
func (m model) detailContent(p pal, a store.Application, paneW int) string {
	if m.detail.tab != tabDocs {
		// The shared overflow cue, on the line under the text: what is below while there is
		// more, then what is above once at the end.
		cue := ""
		if b := m.detail.body; b.TotalLineCount() > b.Height() {
			if below := b.TotalLineCount() - b.YOffset() - b.Height(); below > 0 {
				cue = p.MoreCue(false, below)
			} else {
				cue = p.MoreCue(true, b.YOffset())
			}
		}
		return m.detail.body.View() + "\n" + cue
	}
	var lines []string
	group := ""
	// The state column is as wide as its longest state and a gutter, so a long one ("not
	// needed · Resume only") never runs into the file name after it.
	stateW := docColumn
	for _, e := range docEntries(a) {
		if s, _ := e.displayStatus(a); xansi.StringWidth(s)+gutter > stateW {
			stateW = xansi.StringWidth(s) + gutter
		}
	}
	for i, e := range docEntries(a) {
		if g := e.group(); g != group {
			if group != "" {
				lines = append(lines, "")
			}
			group = g
			lines = append(lines, p.Heading(g))
		}
		status, ready := e.displayStatus(a)
		mark, nameC := p.Text(p.Ok).Render("✓ "), p.Fg
		if !ready {
			mark, nameC = p.Text(p.Dim).Render("○ "), p.Dim
		}
		// The name, then its state, then the file it lives in, muted: what e opens.
		name := e.title()
		file := ""
		if e.markdown() {
			name = padRight(name, docColumn) + padRight(status, stateW)
			file = e.name
		}
		row := "  " + mark + p.Text(nameC).Render(name) + p.Text(p.Dim).Render(file)
		if i == m.detail.docs {
			row = p.SelLine(true, paneW-paneMargin, p.SelGutter(true)+p.SelText(true).Render(xansi.Strip(mark)+name)+
				p.SelBase(true).Foreground(p.Sub).Render(file))
		}
		lines = append(lines, row)
	}
	return strings.Join(lines, "\n")
}

// detailCard is the column beside the tabs: the preview as it stood beside the list,
// with the link under it, h lines tall.
func (m model) detailCard(p pal, a store.Application, h int) string {
	if a.Malformed != nil {
		return ""
	}
	w := m.cardWidth()
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).Padding(0, 1).Render(m.previewText(p, a, w-previewInset, true))
}
