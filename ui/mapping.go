package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// Screens, numbered 1-6.
const (
	screenLeads    = "Leads"
	screenDraft    = "Draft"
	screenApplied  = "Applied"
	screenStats    = "Stats"
	screenProfile  = "Profile"
	screenSettings = "Settings"
)

// profileScreen is the Profile's index in screenDefs.
const profileScreen = 4

// settingsScreen is the Settings' index in screenDefs.
const settingsScreen = 5

// Tables: the views within a screen. Applied, Interviewing and Offer are named after
// their one Status.
const (
	tableLeads        = "Leads"
	tableDismissed    = "Dismissed"
	tableDraft        = "Draft"
	tableApplied      = store.StatusApplied
	tableInterviewing = store.StatusInterviewing
	tableOffer        = store.StatusOffer
	tableClosed       = "Closed"
)

// A screen declares itself: its name, the views within it, whether it has Application
// tables, and — when it has none — the view, keys and help it owns instead, plus the
// refresh it runs when the Owner switches to it. A new screen joins here rather than
// adding a check to the shared layout.
type screenDef struct {
	name      string
	tables    []string
	hasTables bool
	render    func(model, pal) string
	key       func(model, tea.KeyPressMsg) (tea.Model, tea.Cmd)
	help      func(model) []helpKey
	enter     func(model) model
}

// screenDefs is the screens in key order. Stats reports on Applications and Profile
// reads the Owner's fact set rather than listing Applications, so neither has tables
// and each brings its own view. It is filled in init so the handlers can be declared
// beside the screen without an initialisation cycle.
var screenDefs []screenDef

func init() {
	screenDefs = []screenDef{
		{name: screenLeads, tables: []string{tableLeads, tableDismissed}, hasTables: true},
		{name: screenDraft, tables: []string{tableDraft}, hasTables: true},
		{name: screenApplied, tables: []string{allView, tableApplied, tableInterviewing, tableOffer, tableClosed}, hasTables: true},
		{name: screenStats, hasTables: false, render: model.statsView, key: model.statsKey, help: model.statsHelpKeys},
		{name: screenProfile, hasTables: false, render: model.profileView, key: model.profileKey, help: model.profileHelpKeys, enter: model.loadProfile},
		{name: screenSettings, hasTables: false, render: model.settingsView, key: model.settingsKey, help: model.settingsHelpKeys},
	}
}

// screenKey switches screens on 1-6. It lives here rather than in each screen's key
// handler so table, Stats and Profile screens all accept the same digits.
func (m model) screenKey(k tea.KeyPressMsg) (model, bool) {
	key := k.String()
	last := byte('0') + byte(len(screenDefs))
	if len(key) != 1 || key[0] < '1' || key[0] > last {
		return m, false
	}
	m.screen, m.table, m.cursor = int(key[0]-'1'), 0, 0
	m.back = nil
	// Switching screens leaves any open detail behind, so its keys cannot shadow the
	// new screen's.
	m.detail = nil
	if enter := screenDefs[m.screen].enter; enter != nil {
		m = enter(m)
	}
	if m.screen == settingsScreen {
		m = m.loadSettings()
	}
	return m, true
}

// views returns the views within the active screen.
func (m model) views() []string { return screenDefs[m.screen].tables }

// hasTables reports whether the active screen holds Application rows. Screens without
// them have no rows and no view row, and handle their own keys.
func (m model) hasTables() bool { return screenDefs[m.screen].hasTables }

// StatusLocation maps an Application Status to its screen and table.
func StatusLocation(status string) (screen, table string) {
	switch status {
	case store.StatusDismissed:
		return screenLeads, tableDismissed
	case store.StatusDraft:
		return screenDraft, tableDraft
	case store.StatusApplied, store.StatusInterviewing, store.StatusOffer:
		return screenApplied, status
	case store.StatusRejected, store.StatusWithdrawn:
		return screenApplied, tableClosed
	default: // Lead, and anything unreadable
		return screenLeads, tableLeads
	}
}
