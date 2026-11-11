// Package ui implements the Owner's terminal Workspace browser.
package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	huh "charm.land/huh/v2"
	"fmt"
	"github.com/lukebrown14-code/rolecall/store"
	"strings"
	"time"
)

var screens = []string{"Leads", "Draft", "Applied"}
var tables = [][]string{{"Leads", "Dismissed"}, {"Draft"}, {"Applied", "Interviewing", "Offer", "Closed"}}

type binding struct {
	key.Binding
	tablesOnly bool
}

func keys() []binding {
	return []binding{
		{key.NewBinding(key.WithKeys("1", "2", "3"), key.WithHelp("1/2/3", "screens")), false},
		{key.NewBinding(key.WithKeys("tab", "shift+tab"), key.WithHelp("Tab/Shift-Tab", "tables")), true},
		{key.NewBinding(key.WithKeys("up", "k", "down", "j", "g", "G"), key.WithHelp("↑↓/j/k/g/G", "cursor")), false},
		{key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "rescan")), false},
		{key.NewBinding(key.WithKeys("?"), key.WithHelp("?", "full help")), false},
		{key.NewBinding(key.WithKeys("q", "ctrl+c"), key.WithHelp("q/Ctrl-C", "quit")), false},
	}
}

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
}

// New loads a Workspace. The clock is injected for subsequent date-sensitive views.
func New(s *store.Store, now func() time.Time, options ...Option) (tea.Model, error) {
	a, e := s.Scan()
	if e != nil {
		return nil, e
	}
	if now == nil {
		now = time.Now
	}
	tracks, e := s.Tracks()
	if e != nil {
		return nil, e
	}
	m := model{store: s, apps: a, tracks: tracks, now: now, followUpDays: 14}
	for _, option := range options {
		option(&m)
	}
	return m, nil
}
func (m model) Init() tea.Cmd { return nil }
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
			return m, tea.Quit
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
					previous := "Lead"
					if len(a.StatusLog) > 1 {
						previous = a.StatusLog[len(a.StatusLog)-2].Status
					}
					m.options = MoveOptions(a.Status, previous)
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
			return m, tea.Quit
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
		case "r":
			folder := ""
			rows := m.rows()
			if len(rows) > 0 {
				folder = rows[m.cursor].Folder
			}
			a, e := m.store.Scan()
			m.err = e
			if e == nil {
				m.apps = a
				for i, r := range m.rows() {
					if r.Folder == folder {
						m.cursor = i
						break
					}
				}
			}
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
	if len(m.tracks) > 0 {
		track := "All"
		if m.track > 0 {
			track = m.tracks[m.track-1]
		}
		b.WriteString("Track: " + track + " · t track  ")
	}
	b.WriteString("/ filter · Esc clear")
	if m.text != "" || m.filtering {
		b.WriteString(" · Filter: " + m.text)
	}
	b.WriteString("\n")
	if len(m.options) > 0 {
		b.WriteString(m.moveView())
	}
	if m.toast != "" {
		b.WriteString(m.toast + "\n")
	}
	if len(m.rows()) > 0 {
		b.WriteString("m move · u undo")
		a := m.rows()[m.cursor]
		if a.Status == "Lead" || a.Status == "Draft" {
			b.WriteString(" · x dismiss")
		}
		b.WriteString("\n")
	}
	if m.err != nil {
		b.WriteString(m.err.Error() + "\n")
	}
	if len(m.rows()) > 0 {
		b.WriteString("n notes  ")
	}
	if m.full {
		b.WriteString("Full help\n")
	}
	for _, k := range keys() {
		if k.tablesOnly && len(tables[m.screen]) == 1 {
			continue
		}
		h := k.Help()
		b.WriteString(h.Key + " " + h.Desc + "  ")
		if m.full {
			b.WriteString("\n")
		}
	}
	return tea.NewView(plainStyle.Render(b.String()))
}
