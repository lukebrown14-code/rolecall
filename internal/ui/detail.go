package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"time"

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
	present bool
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
const resumeOnlyNote = "not needed · Resume only"

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
	entries := []docEntry{{jdFile, a.HasJD, a.JDHasText}, {resumeFile, a.HasResume, a.ResumeHasText}, {coverLetterFile, a.HasCoverLetter, a.CoverLetterHasText}, {gapsFile, a.HasGaps, a.GapsHasText}}
	for _, pdf := range a.PDFs {
		entries = append(entries, docEntry{"out/" + pdf, true, false})
	}
	if a.HasSent {
		entries = append(entries, docEntry{"sent/", true, false})
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

func (m *model) openDetail(folder string) {
	m.detail = &detailState{folder: folder, tab: tabJD, body: viewport.New()}
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
	paneChrome    = 3  // the pane's lines around the tab content: tabs row, gap … scroll cue
	maxReadWidth  = 80 // long text wraps at or before this
	minWrapWidth  = 20
	paneMargin    = 4
	docColumn     = 14        // the Docs tab's name and state columns
	stripLines    = 2         // the strip that stands in for the card: timeline track, then documents
	cardPadding   = inset + 1 // the card's padding: the text column on the left, one cell on the right
)

// cardShown reports whether the detail view has room for the left card: the wide size
// class. Below it the card folds into a strip under the title.
func (m model) cardShown() bool { return m.layout().wide() }

// cardWidth is the left card's column, divider excluded.
func (m model) cardWidth() int { return m.layout().panel(cardShare, cardMin, cardMax) }

// detailContextLines is the lines between the tab bar and the body: the title row, then
// the facts line beside the card or the strip in its place.
func (m model) detailContextLines() int {
	if m.cardShown() {
		return 2 //nolint:mnd // title and facts
	}
	return 1 + stripLines
}

// paneWidth is the right-hand pane: everything beside the card.
func (m model) paneWidth() int {
	if m.cardShown() {
		return m.viewWidth() - m.cardWidth() - 1
	}
	return m.viewWidth()
}

// bodyHeight is the lines of tab content shown at once.
func (m model) bodyHeight() int {
	return max(minBodyHeight, m.layout().bodyHeight(m.detailContextLines())-paneChrome)
}

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
	if !slices.Contains(visibleDetailTabs(a), d.tab) {
		m.err = fmt.Errorf("%s is gone", docName(markdownTabFiles[d.tab]))
		d.tab = tabDocs
	}
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
	theme theme
	r     *glamour.TermRenderer
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

// markdownBody renders one of the Markdown tabs, or its empty state, which says what to
// press to fill it.
func (m model) markdownBody(a store.Application, tab detailTab, name string) string {
	var out string
	b, err := m.store.ReadDocument(a.Folder, name)
	empty := err != nil || strings.TrimSpace(string(b)) == ""
	if tab == tabFit {
		out = "Fit: " + fitSummary(a, !empty) + "\n\n"
	}
	if empty {
		return out + m.emptyDocument(a, tab, name)
	}
	return out + m.renderMarkdown(string(b), m.bodyWidth())
}

// emptyDocument is a tab's empty state: what is missing and the key that fills it.
func (m model) emptyDocument(a store.Application, tab detailTab, name string) string {
	switch tab {
	case tabJD:
		return "No JD saved · e to open " + name
	case tabFit:
		if m.harness != "" {
			return "No fit assessment yet · f to assess fit"
		}
		return "No fit assessment yet"
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
		return fitNone
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
		if rendered, err := r.Render(text); err == nil {
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
	case "right", "left":
		tabs := visibleDetailTabs(a)
		step := 1
		if key == "left" {
			step = len(tabs) - 1
		}
		d.tab = tabs[(slices.Index(tabs, d.tab)+step)%len(tabs)]
		d.body.GotoTop()
		return m, nil, true
	case "m", "n", "d", "u", "r", "b", "f", "p", "?", "q", "ctrl+c",
		// screen digits fall through to screenKey, which closes the detail
		"1", "2", "3", "4", "5":
		return m, nil, false
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
	switch key {
	case "down", "j":
		d.body.ScrollDown(1)
	case "up", "k":
		d.body.ScrollUp(1)
	case "pgdown", "space":
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
		case e.markdown() && key == "e":
			return m, m.editFile(a.Folder, e.name), true
		case e.markdown():
			m.err = fmt.Errorf("%s is missing · e to open %s", e.title(), e.name)
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
	title := pad + p.Text(p.Dim).Render("‹ esc  ") + p.Text(p.Fg).Bold(true).Render(appName(a.Title)) + p.Text(p.Dim).Render(" · "+a.Company)
	when := ""
	switch text, s := closeInfo(a.Closes, m.now()); {
	case a.Malformed != nil:
		title = pad + p.Text(p.Dim).Render("‹ esc  ") + p.Text(p.Bad).Bold(true).Render("⚠ "+m.malformedTitle(a))
	case s != closeNone && beforeApplied(tableOf(a)):
		when = p.CloseStyle(s, p.Text, p.Dim).Render(text)
	case a.AppliedDate != nil:
		if m.futureDate(a.AppliedDate) {
			when = p.Text(p.Warn).Render("applied " + futureFlag)
		} else {
			// The exact applied date shows once: here beside the card, on the strip's
			// timeline track without it.
			exact := ""
			if m.cardShown() {
				exact = entryDate(a.AppliedDate) + " · "
			}
			rel := m.agoEntry(a.AppliedDate)
			if _, days, _ := daysFrom(a.AppliedDate.At, m.now()); exact != "" && -days >= monthYearAfter {
				// An old date already reads as a month: the exact date says it all.
				exact, rel = strings.TrimSuffix(exact, " · "), ""
			}
			when = p.Text(p.Dim).Render("applied " + exact + rel)
		}
	}
	when = m.withChip(p, when)
	title = spread(xansi.Truncate(title, w-xansi.StringWidth(when)-ellipsis, "…"), when, w-rightMargin)

	// Beside the card, the facts line: Status · Track · close date. Without the card, the
	// strip stands in for it.
	closes := a.Closes
	if closes == "" {
		closes = "—"
	}
	facts := pad + p.StatusChip(a.Status) + p.Text(p.Dim).Render(" · "+a.Track+" · closes "+closes)
	ctx := []string{title, facts}
	if !m.cardShown() {
		ctx = append([]string{title}, m.detailStrip(p, a, w-inset-rightMargin)...)
	}
	var pane string
	if a.Malformed != nil {
		for i := 1; i < len(ctx); i++ {
			ctx[i] = ""
		}
		// Where and how to fix it, the file's Workspace path, and the key that opens it there.
		pane = p.Text(p.Fg).Render(m.malformedReason(a)) + "\n\n" + p.Text(p.Dim).Render(m.rel(filepath.Join(a.Folder, "meta.yml"))) +
			"\n\n" + p.Key().Render("e") + p.Text(p.Dim).Render(" fix in editor")
	} else {
		// The tab names sit one cell in from their brackets; the content lines up with them.
		content := strings.Split(m.detailContent(p, a, paneW), "\n")
		for i, l := range content {
			content[i] = " " + l
		}
		// Without the card, the next step takes the line between the tabs and the text.
		gap := ""
		if !m.cardShown() {
			gap = " " + m.nextLine(p, a, paneW-gutter-1)
		}
		pane = m.detailTabsRow(p, a) + "\n" + gap + "\n" + strings.Join(content, "\n")
	}
	pane = lipgloss.NewStyle().Width(paneW).Height(h).MaxHeight(h).PaddingLeft(gutter).Render(pane)
	body, split, focus := pane, 0, focusNone
	if m.cardShown() {
		cardW := m.cardWidth()
		card := lipgloss.NewStyle().Width(cardW).Height(h).MaxHeight(h).Render(m.detailCard(p, a))
		// The tab pane scrolls and takes the keys, so its rule is the accent.
		body, split, focus = lipgloss.JoinHorizontal(lipgloss.Top, card, p.Divider(h), pane), cardW, focusRight
	}
	return m.frame(p, ctx, body, split, focus, m.statusRow(p, w, m.keyLine(p, w)))
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
	fit := a.Fit
	if fit == "" {
		fit = fitNone
	}
	parts := []string{m.fileMarks(p, a), p.Text(p.Dim).Render("fit ") + p.Text(p.FitColor(a.Fit)).Render(fit)}
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

// timelineFlow is the track for a narrow column: every stop with its date, flowing onto
// as many lines as w needs, each line ending in the rule that leads to the next.
func (m model) timelineFlow(p pal, a store.Application, w int) string {
	_, pastDated, current, ahead := m.trackStops(p, a)
	stops := append(append(append([]string{}, pastDated...), current), ahead...)
	sep := p.Text(p.Rule).Render(" ─ ")
	var lines []string
	line := ""
	for i, s := range stops {
		item := s
		if i < len(stops)-1 {
			item += p.Text(p.Rule).Render(" ─")
		}
		switch {
		case line == "":
			line = item
		case xansi.StringWidth(strings.TrimSuffix(line, p.Text(p.Rule).Render(" ─"))+sep+item) <= w:
			line = strings.TrimSuffix(line, p.Text(p.Rule).Render(" ─")) + sep + item
		default:
			lines = append(lines, line)
			line = item
		}
	}
	return strings.Join(append(lines, line), "\n")
}

// trackStops is a timeline track's stops: earlier Statuses without and with their dates,
// the current one, and the Statuses still ahead.
func (m model) trackStops(p pal, a store.Application) (past, pastDated []string, current string, ahead []string) {
	date := func(e store.StatusEntry) string {
		if m.futureDate(&e) {
			return p.Text(p.Warn).Render(" " + futureFlag)
		}
		t, err := time.Parse("2006-01-02", strings.Split(e.At, "T")[0])
		if err != nil {
			return ""
		}
		d := shortDate(t, m.now())
		if e.Approx {
			d = "~" + d
		}
		return p.Text(p.Dim).Render(" " + d)
	}
	for i, e := range a.StatusLog {
		if i == len(a.StatusLog)-1 {
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

func (m model) detailTabsRow(p pal, a store.Application) string {
	visible := visibleDetailTabs(a)
	tabs := make([]string, len(visible))
	for i, t := range visible {
		tabs[i] = p.Text(p.Dim).Render(" " + string(t) + " ")
		if t == m.detail.tab {
			// Bracketed as well as filled: the active tab is marked without colour.
			tabs[i] = p.ActiveTab().Render("[" + string(t) + "]")
		}
	}
	return strings.Join(tabs, " ")
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
			name = padRight(name, docColumn) + padRight(status, docColumn)
			file = e.name
		}
		row := "  " + mark + p.Text(nameC).Render(name) + p.Text(p.Faint).Render(file)
		if i == m.detail.docs {
			row = lipgloss.NewStyle().Background(p.Sel).Width(paneW - paneMargin).Render(
				p.Text(p.Accent).Background(p.Sel).Render("▌ ") + p.Text(p.Accent).Background(p.Sel).Bold(true).Render(xansi.Strip(mark)+name) +
					p.Text(p.Sub).Background(p.Sel).Render(file))
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
	inner := m.cardWidth() - cardPadding
	b.WriteString(keepTogether(inner, "", p.StatusChip(a.Status), p.FitText(a)) + "\n\n")
	if next := m.nextLine(p, a, inner); next != "" {
		b.WriteString(next + "\n\n")
	}
	b.WriteString(p.Heading("Docs") + "\n" + p.DocsMeter(a) + "\n")
	for _, c := range docChips(a) {
		b.WriteString(m.chipText(p, a, c) + "\n")
	}
	if a.ResumeOnly && !a.CoverLetterHasText {
		b.WriteString(p.Text(p.Dim).Render("  "+docCoverLetter+"\n  "+resumeOnlyNote) + "\n")
	}
	b.WriteString("\n" + p.Heading("Timeline") + "\n" + m.timelineBlock(p, a, inner, true) + "\n")
	if a.URL != "" {
		b.WriteString("\n" + p.Heading("Link") + "\n" + p.Text(p.Info).Render(xansi.Truncate(a.URL, m.cardWidth()-cardPadding, "…")))
	}
	return lipgloss.NewStyle().Width(m.cardWidth()).Padding(0, 1, 0, inset).Render(strings.TrimSuffix(b.String(), "\n"))
}
