package ui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/glamour/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

const (
	tabJD = iota
	tabNotes
	tabFit
	tabDocs
)

var detailTabs = []string{"JD", "Notes", "Fit", "Docs"}

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
func (e docEntry) pdf() bool      { return strings.HasSuffix(e.name, ".pdf") }

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

func (m *model) moveCurrent(status string) {
	if a, ok := m.current(); ok {
		m.requestMove(a, status)
	}
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

// bodyHeight leaves room for the header, tab bar, footer and Move popup.
func (m model) bodyHeight() int {
	if m.height == 0 {
		return 20
	}
	return max(3, m.height-8)
}

func (m model) bodyWidth() int {
	if m.width == 0 {
		return 80
	}
	return m.width
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

// markdownBody renders one of the Markdown tabs, or its empty state.
func (m model) markdownBody(a store.Application, tab int, name string) string {
	var out string
	if tab == tabFit {
		fit := a.Fit
		if fit == "" {
			fit = "—"
		}
		out = "Fit: " + fit + "\n\n"
	}
	b, err := m.store.ReadDocument(a.Folder, name)
	if err != nil || strings.TrimSpace(string(b)) == "" {
		return out + "No content in " + name
	}
	r, err := glamour.NewTermRenderer(glamour.WithStyles(markdownStyle()), glamour.WithWordWrap(max(20, m.bodyWidth()-4)))
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
	case "m", "n", "d", "u", "r", "?", "q", "ctrl+c":
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
	case "pgup", "b":
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
			m.toast = e.name + " does not exist"
		}
	}
	return m, nil, true
}

func (m model) detailView() string {
	a, ok := m.application(m.detail.folder)
	if !ok {
		return ""
	}
	var b strings.Builder
	if a.Malformed != nil {
		fmt.Fprintf(&b, "%s\n%s\n", headerStyle.Render("⚠ "+filepath.Join(a.Folder, "meta.yml")), a.Malformed)
	} else {
		closes, applied := a.Closes, entryDate(a.AppliedDate)
		if closes == "" {
			closes = "—"
		}
		if applied == "" {
			applied = "—"
		}
		fmt.Fprintf(&b, "%s\n%s · %s · closes %s · applied %s\n", headerStyle.Render(a.Company+" · "+a.Title), a.Status, a.Track, closes, applied)
		tabs := make([]string, len(detailTabs))
		for i, t := range detailTabs {
			tabs[i] = t
			if i == m.detail.tab {
				tabs[i] = selectedStyle.Render(t)
			}
		}
		b.WriteString(strings.Join(tabs, " · ") + "\n")
		if m.detail.tab == tabDocs {
			for i, e := range docEntries(a) {
				line := e.label()
				if i == m.detail.docs {
					line = selectedStyle.Render(line)
				}
				b.WriteString(line + "\n")
			}
		} else {
			b.WriteString(m.detail.body.View() + "\n")
		}
	}
	if len(m.options) > 0 {
		b.WriteString(m.moveView())
	}
	if m.confirm != nil {
		b.WriteString("Docs not Ready. Mark Applied anyway? y/n\n")
	}
	b.WriteString(m.downloadPrompt())
	if m.toast != "" {
		b.WriteString(m.toast + "\n")
	}
	if m.err != nil {
		b.WriteString(m.err.Error() + "\n")
	}
	b.WriteString("←/→ tabs · ↑↓ scroll · e edit · m move · n notes")
	if len(a.PDFs) > 0 {
		b.WriteString(" · d download")
	}
	b.WriteString(" · u undo · Esc back")
	return plainStyle.Render(b.String())
}
