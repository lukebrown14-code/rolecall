// Package ui implements the Owner's terminal Workspace browser.
package ui

import (
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"fmt"
	"github.com/lukebrown14-code/rolecall/store"
	"path/filepath"
	"strings"
	"time"
)

var screens = []string{"Leads", "Draft", "Applied"}
var tables = [][]string{{"Leads", "Dismissed"}, {"Draft"}, {"Applied", "Interviewing", "Offer", "Closed"}}

type model struct {
	store                 *store.Store
	apps                  []store.Application
	screen, table, cursor int
	tracks                []string
	track                 int
	text                  string
	filtering             bool
	full                  bool
	options               []string
	moveForm              *huh.Form
	moveTarget            string
	addForm               *huh.Form
	addValues             *addValues
	editOf                *store.Application
	duplicate             *store.Application
	duplicateFields       store.CreateFields
	selectAfter           string
	confirm               *pendingMove
	undo                  []undoMove
	toast                 string
	err                   error
	width                 int
	followUpDays          int
	now                   func() time.Time
	editor                string
	external              ExternalRunner
	detail                *detailState
	height                int
	watchWindow           time.Duration
	watch                 *watcher
	downloads             string
	pendingDownload       *downloadState
}

// New loads a Workspace. The clock is injected for subsequent date-sensitive views.
func New(s *store.Store, now func() time.Time, options ...Option) (tea.Model, error) {
	if now == nil {
		now = time.Now
	}
	m := model{store: s, now: now, followUpDays: 14}
	for _, option := range options {
		option(&m)
	}
	if m.watchWindow > 0 {
		// Watching starts before the first scan so no change slips between the two.
		w, e := startWatcher(filepath.Join(s.Root(), "jobs"), m.watchWindow)
		if e != nil {
			m.err = fmt.Errorf("live refresh unavailable: %w", e)
		}
		m.watch = w
	}
	a, e := s.Scan()
	if e == nil {
		m.apps = a
		m.tracks, e = s.Tracks()
	}
	if e != nil {
		m.watch.close()
		return nil, e
	}
	return m, nil
}
func (m model) Init() tea.Cmd {
	if m.watch == nil {
		return nil
	}
	return m.watch.next()
}

// quit stops the watcher, then quits.
func (m model) quit() tea.Cmd {
	return func() tea.Msg {
		m.watch.close()
		return tea.QuitMsg{}
	}
}
func (m model) rows() []store.Application {
	var out []store.Application
	for _, a := range m.apps {
		s, t := StatusLocation(a.Status)
		if s == screens[m.screen] && t == tables[m.screen][m.table] && m.matches(a) {
			out = append(out, a)
		}
	}
	SortApplications(out, tables[m.screen][m.table])
	return out
}
func (m model) matches(a store.Application) bool {
	return (m.track == 0 || a.Track == m.tracks[m.track-1]) && strings.Contains(strings.ToLower(a.Title+" "+a.Company), strings.ToLower(m.text))
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
	}
	if batch, ok := msg.(foldersChanged); ok {
		return m.refresh(batch.folders), m.watch.next()
	}
	if done, ok := msg.(externalFinished); ok {
		a, err := m.store.Rescan(done.folder)
		m.err = done.err
		if err != nil {
			m.err = err
		} else if a != nil {
			for i := range m.apps {
				if m.apps[i].Folder == done.folder {
					m.apps[i] = *a
					break
				}
			}
		}
		if done.folder == m.selectAfter {
			m.selectFolder(done.folder)
			m.selectAfter = ""
		}
	}
	if m.addForm != nil {
		return m.updateAdd(msg)
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && m.duplicate != nil {
		return m.updateDuplicate(k)
	}
	if m.moveForm != nil {
		if _, key := msg.(tea.KeyPressMsg); !key {
			updated, cmd := m.moveForm.Update(msg)
			m.moveForm = updated.(*huh.Form)
			if m.moveForm.State == huh.StateCompleted {
				m.moveCurrent(m.moveForm.GetString("status"))
				m.options = nil
				m.moveForm = nil
			}
			return m, cmd
		}
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if k.String() == "ctrl+c" {
			return m, m.quit()
		}
		if m.pendingDownload != nil {
			return m, m.answerDownload(k.String())
		}
		if m.confirm != nil {
			pending := m.confirm
			switch k.String() {
			case "y":
				m.confirm = nil
				m.move(pending.app, pending.status, false)
			case "n", "esc":
				m.confirm = nil
			}
			return m, nil
		}
		if m.filtering {
			switch k.String() {
			case "esc":
				m.text = ""
				m.filtering = false
			case "enter":
				m.filtering = false
			case "backspace":
				r := []rune(m.text)
				if len(r) > 0 {
					m.text = string(r[:len(r)-1])
				}
			default:
				if k.Text != "" {
					m.text += k.Text
				}
			}
			m.cursor = 0
			return m, nil
		}
		if len(m.options) > 0 {
			if k.String() == "esc" {
				m.options = nil
				m.moveForm = nil
				return m, nil
			}
			for _, status := range m.options {
				if statusLetters[status] == k.String() {
					m.moveCurrent(status)
					m.options = nil
					m.moveForm = nil
					return m, nil
				}
			}
			updated, cmd := m.moveForm.Update(msg)
			m.moveForm = updated.(*huh.Form)
			if m.moveForm.State == huh.StateCompleted {
				m.moveCurrent(m.moveForm.GetString("status"))
				m.options = nil
				m.moveForm = nil
			}
			return m, cmd
		}

		if m.detail != nil {
			if next, cmd, handled := m.detailKey(k); handled {
				return next.syncDetail(), cmd
			}
		}
		switch k.String() {
		case "v", "enter":
			if rows := m.rows(); len(rows) > 0 {
				m.openDetail(rows[m.cursor].Folder)
			}
		case "m", "x":
			if a, ok := m.current(); ok {
				if k.String() == "x" {
					if a.Status == "Lead" || a.Status == "Draft" {
						m.move(a, "Dismissed", false)
					}
				} else {
					m.options = MoveOptions(a.Status, statusBeforeDismissed(a.StatusLog))
					opts := make([]huh.Option[string], 0, len(m.options))
					for _, status := range m.options {
						opts = append(opts, huh.NewOption(statusLetters[status]+" "+status, status))
					}
					m.moveForm = huh.NewForm(huh.NewGroup(huh.NewSelect[string]().Key("status").Title("Move").Options(opts...))).WithShowHelp(false).WithShowErrors(false)
					return m, m.moveForm.Init()
				}
			}
		case "u":
			if len(m.undo) > 0 {
				last := m.undo[len(m.undo)-1]
				for _, a := range m.apps {
					if a.Folder == last.folder {
						m.move(a, last.status, true)
						if m.err == nil {
							m.undo = m.undo[:len(m.undo)-1]
						}
						break
					}
				}
			}

		case "a":
			return m, m.openAdd()
		case "e":
			if a, ok := m.current(); ok {
				return m, m.openEdit(a)
			}
		case "t":
			if len(m.tracks) > 0 {
				m.track = (m.track + 1) % (len(m.tracks) + 1)
				m.cursor = 0
			}
		case "/":
			m.filtering = true
		case "esc":
			m.text = ""
			m.cursor = 0
		case "q", "ctrl+c":
			return m, m.quit()
		case "1", "2", "3":
			m.screen = int(k.String()[0] - '1')
			m.table = 0
			m.cursor = 0
		case "tab":
			m.table = (m.table + 1) % len(tables[m.screen])
			m.cursor = 0
		case "shift+tab":
			m.table = (m.table + len(tables[m.screen]) - 1) % len(tables[m.screen])
			m.cursor = 0
		case "down", "j":
			m.cursor++
		case "up", "k":
			m.cursor--
		case "g":
			m.cursor = 0
		case "G":
			m.cursor = len(m.rows()) - 1
		case "?":
			m.full = !m.full
		case "n":
			if a, ok := m.current(); ok {
				return m, m.editNotes(a.Folder)
			}
		case "d":
			if a, ok := m.current(); ok && len(a.PDFs) > 0 {
				return m, m.download(a)
			}
		case "r":
			m.rescanAll()
		}
	}
	m = m.syncDetail()
	n := len(m.rows())
	if m.cursor >= n {
		m.cursor = n - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
	return m, nil
}
func (m model) View() tea.View {
	if m.detail != nil {
		return tea.NewView(m.detailView())
	}
	var b strings.Builder
	b.WriteString(headerStyle.Render(screens[m.screen]) + "\n")
	for i, t := range tables[m.screen] {
		count := 0
		for _, a := range m.apps {
			s, tbl := StatusLocation(a.Status)
			if s == screens[m.screen] && tbl == t && m.matches(a) {
				count++
			}
		}
		label := fmt.Sprintf("%s (%d)", t, count)
		if i == m.table {
			label = headerStyle.Render(label)
		}
		b.WriteString(label + "  ")
	}
	table := tables[m.screen][m.table]
	headers := []string{"Title", "Company", "Track"}
	switch table {
	case "Leads":
		headers = append(headers, "Closes", "Fit")
	case "Draft":
		headers = append(headers, "Closes", "Fit", "Docs")
	case "Dismissed":
		headers = append(headers, "Dismissed date")
	default:
		headers = append(headers, "Applied", "Fit", "Docs", "Follow-up")
	}
	cells := [][]string{headers}
	for _, a := range m.rows() {
		cells = append(cells, ApplicationColumns(a, table, m.now(), m.followUpDays))
	}
	lines := RenderColumns(cells, m.width)
	b.WriteString("\n" + headerStyle.Render(lines[0]) + "\n")
	for i, row := range lines[1:] {
		if i == m.cursor {
			row = selectedStyle.Render(row)
		}
		b.WriteString(row + "\n")
	}
	if line := m.filterLine(); line != "" {
		b.WriteString(line + "\n")
	}
	if len(m.options) > 0 {
		b.WriteString(m.moveView())
	}
	b.WriteString(m.addView())
	if m.confirm != nil {
		b.WriteString("Docs not Ready. Mark Applied anyway? y/n\n")
	}
	b.WriteString(m.downloadPrompt())
	if m.toast != "" {
		b.WriteString(m.toast + "\n")
	}
	b.WriteString(m.rowHelp())
	if m.err != nil {
		b.WriteString(m.err.Error() + "\n")
	}
	b.WriteString(m.otherHelp())
	return tea.NewView(plainStyle.Render(b.String()))
}

// cursorFolder is the folder of the row under the cursor, or "" when the table is empty.
func (m model) cursorFolder() string {
	if rows := m.rows(); m.cursor >= 0 && m.cursor < len(rows) {
		return rows[m.cursor].Folder
	}
	return ""
}

// keepCursor puts the cursor on folder's row when the active table still shows it.
func (m *model) keepCursor(folder string) {
	for i, r := range m.rows() {
		if r.Folder == folder {
			m.cursor = i
			return
		}
	}
}

// reload rescans the whole Workspace and keeps the cursor on folder when it is still shown.
// A failed scan is reported through m.err and leaves the rows as they were.
func (m *model) reload(folder string) bool {
	apps, err := m.store.Scan()
	if err != nil {
		m.err = err
		return false
	}
	m.apps, m.err = apps, nil
	m.keepCursor(folder)
	return true
}

// rescanAll is `r`: it rereads workspace.json as well as every Application.
func (m *model) rescanAll() {
	folder := m.cursorFolder()
	tracks, err := m.store.Tracks()
	if err != nil {
		m.err = err
		return
	}
	m.tracks = tracks
	if m.track > len(tracks) {
		m.track = 0 // the filtered track is gone
	}
	m.reload(folder)
}
