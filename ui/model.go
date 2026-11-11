// Package ui implements the Owner's terminal Workspace browser.
package ui

import (
	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
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
	full                  bool
	err                   error
	width                 int
	followUpDays          int
	now                   func() time.Time
}

// New loads a Workspace. The clock is injected for subsequent date-sensitive views.
func New(s *store.Store, now func() time.Time, followUpDays ...int) (tea.Model, error) {
	a, e := s.Scan()
	if e != nil {
		return nil, e
	}
	if now == nil {
		now = time.Now
	}
	days := 14
	if len(followUpDays) > 0 {
		days = followUpDays[0]
	}
	return model{store: s, apps: a, now: now, followUpDays: days}, nil
}
func (m model) Init() tea.Cmd { return nil }
func (m model) rows() []store.Application {
	var out []store.Application
	for _, a := range m.apps {
		s, t := StatusLocation(a.Status)
		if s == screens[m.screen] && t == tables[m.screen][m.table] {
			out = append(out, a)
		}
	}
	sortApplications(out, tables[m.screen][m.table])
	return out
}
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width = size.Width
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
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
	var b strings.Builder
	b.WriteString(headerStyle.Render(screens[m.screen]) + "\n")
	for i, t := range tables[m.screen] {
		count := 0
		for _, a := range m.apps {
			s, tbl := StatusLocation(a.Status)
			if s == screens[m.screen] && tbl == t {
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
	lines := renderColumns(cells, m.width)
	b.WriteString("\n" + headerStyle.Render(lines[0]) + "\n")
	for i, row := range lines[1:] {
		if i == m.cursor {
			row = selectedStyle.Render(row)
		}
		b.WriteString(row + "\n")
	}
	if m.err != nil {
		b.WriteString(m.err.Error() + "\n")
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
