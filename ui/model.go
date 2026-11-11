// Package ui implements the Owner's terminal Workspace browser.
package ui

import (
	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"fmt"
	"github.com/lukebrown14-code/rolecall/store"
	"path/filepath"
	"strings"
	"time"
)

var screens = []string{screenLeads, screenDraft, screenApplied, screenStats}

// allView is the Applied screen's first view: every Application past Draft, one
// section per table in appliedGroups order.
const allView = "All"

var tables = [][]string{{tableLeads, tableDismissed}, {tableDraft}, {allView, tableApplied, tableInterviewing, tableOffer, tableClosed}, {}}

var appliedGroups = []string{tableApplied, tableInterviewing, tableOffer, tableClosed}

// defaultFollowUpDays applies when the Owner configures none.
const defaultFollowUpDays = 14

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
	moveSel               int    // highlighted entry in the Move pop-up
	moveTarget            string // folder the Move pop-up acts on
	form                  *form  // Add/Edit pop-up
	editOf                *store.Application
	duplicate             *store.Application
	duplicateFields       store.CreateFields
	selectAfter           string
	confirm               *pendingMove
	undo                  []undoMove
	toast                 string // feedback line: what just happened (cleared on the next key)
	toastHint             string // key hint shown with it, e.g. "u undo"
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
	md                    *mdCache
	plugin                string
	harness               string
	captured              CapturedRunner
	capturedRun           bool // a build or fetch is in flight; only one captured run at a time
	building              *buildState
	buildFail             *buildFailure
	spinner               spinner.Model
}

// New loads a Workspace. The clock is injected for subsequent date-sensitive views.
func New(s *store.Store, now func() time.Time, options ...Option) (tea.Model, error) {
	if now == nil {
		now = time.Now
	}
	m := model{store: s, now: now, followUpDays: defaultFollowUpDays, md: &mdCache{},
		spinner: spinner.New(spinner.WithSpinner(spinner.MiniDot), spinner.WithStyle(basePal.spinnerStyle()))}
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

// rows is the active view in display order: the cursor indexes into it. Stats has
// no tables and no rows.
func (m model) rows() []store.Application {
	if m.screen == statsScreen {
		return nil
	}
	return m.rowsOf(m.screen, tables[m.screen][m.table])
}

// rowsOf lists one view of a screen. All groups the Applied screen's tables in
// appliedGroups order; Leads and Draft put expired close dates last.
func (m model) rowsOf(screen int, table string) []store.Application {
	if table == allView {
		var out []store.Application
		for _, g := range appliedGroups {
			out = append(out, m.rowsOf(screen, g)...)
		}
		return out
	}
	var out []store.Application
	for _, a := range m.apps {
		s, t := StatusLocation(a.Status)
		if s == screens[screen] && t == table && m.matches(a) {
			out = append(out, a)
		}
	}
	SortApplications(out, table)
	if beforeApplied(table) {
		var live, old []store.Application
		for _, a := range out {
			if m.expired(a) {
				old = append(old, a)
			} else {
				live = append(live, a)
			}
		}
		out = append(live, old...)
	}
	return out
}

// beforeApplied reports whether a table holds Applications not yet sent: Leads and Draft.
func beforeApplied(table string) bool { return table == tableLeads || table == tableDraft }

func (m model) count(screen int, table string) int { return len(m.rowsOf(screen, table)) }

// totalCount is the Applications in a view ignoring the track and text filters, so the
// header and tab counts stay totals; the scope line shows the filtered number beside them.
func (m model) totalCount(screen int, table string) int {
	if table == allView {
		n := 0
		for _, g := range appliedGroups {
			n += m.totalCount(screen, g)
		}
		return n
	}
	n := 0
	for _, a := range m.apps {
		s, t := StatusLocation(a.Status)
		if s == screens[screen] && t == table {
			n++
		}
	}
	return n
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
	if tick, ok := msg.(spinner.TickMsg); ok {
		if m.building == nil && !m.fetching() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(tick)
		return m, cmd
	}
	if done, ok := msg.(fetchFinished); ok {
		// The form may have been closed while the fetch was in flight: the run is still
		// over, so clear the guard and drop the result rather than leaking it.
		m.capturedRun = false
		if m.form == nil {
			return m, nil
		}
		return m.finishFetch(done)
	}
	if done, ok := msg.(buildFinished); ok {
		m.capturedRun = false
		return m.finishBuild(done)
	}
	if m.form != nil {
		return m.updateAdd(msg)
	}
	if paste, ok := msg.(tea.PasteMsg); ok && m.filtering {
		m.text += pasted(paste.Content)
		m.cursor = 0
		return m, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && m.buildFail != nil {
		switch k.String() {
		case "esc":
			m.buildFail = nil
		case "ctrl+c":
			return m, m.quit()
		}
		return m, nil
	}
	if k, ok := msg.(tea.KeyPressMsg); ok && m.duplicate != nil {
		return m.updateDuplicate(k)
	}
	if k, ok := msg.(tea.KeyPressMsg); ok {
		if k.String() == "ctrl+c" {
			return m, m.quit()
		}
		// The feedback line: a success clears on the next key; an error stays until esc.
		m.toast, m.toastHint = "", ""
		if m.full {
			m.full = false // any key closes the help pop-up, esc included
			return m, nil
		}
		if k.String() == "esc" && m.err != nil && !m.overlayOpen() {
			m.err = nil
			return m, nil
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
				m.text = dropLastGrapheme(m.text)
			default:
				if k.Text != "" {
					m.text += k.Text
				}
			}
			m.cursor = 0
			return m, nil
		}
		if len(m.options) > 0 {
			return m.moveKey(k.String()), nil
		}

		if m.detail != nil {
			if next, cmd, handled := m.detailKey(k); handled {
				return next.syncDetail(), cmd
			}
		}
		if m.screen == statsScreen {
			return m.statsKey(k)
		}
		switch k.String() {
		case "v", "enter":
			if rows := m.rows(); len(rows) > 0 {
				m.openDetail(rows[m.cursor].Folder)
			}
		case "m", "x":
			if a, ok := m.current(); ok {
				if k.String() == "x" {
					if a.Status == store.StatusLead || a.Status == store.StatusDraft {
						m.move(a, store.StatusDismissed, false)
					}
				} else {
					m.options = MoveOptions(a.Status, statusBeforeDismissed(a.StatusLog))
					m.moveSel, m.moveTarget = 0, a.Folder
					return m, nil
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
		case "b":
			return m, m.startBuild()
		case "f":
			return m, m.startHandoff(skillAssessFit)
		case "p":
			return m, m.startHandoff(skillPrepareDocuments)
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
		case "1", "2", "3", "4":
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
			m.full = true
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
