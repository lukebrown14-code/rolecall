package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/store"
)

// Screens, numbered 1-4.
const (
	screenLeads   = "Leads"
	screenDraft   = "Draft"
	screenApplied = "Applied"
	screenStats   = "Stats"
)

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
// tables, and — when it has none — the view, keys and help it owns instead. A new
// screen joins here rather than adding a check to the shared layout.
type screenDef struct {
	name      string
	tables    []string
	hasTables bool
	render    func(model, pal) string
	key       func(model, tea.KeyPressMsg) (tea.Model, tea.Cmd)
	help      func(model) []helpKey
}

// screenDefs is the screens in key order. Stats reports on Applications rather than
// listing them, so it has no tables and brings its own view. It is filled in init so
// the handlers can be declared beside the screen without an initialisation cycle.
var screenDefs []screenDef

func init() {
	screenDefs = []screenDef{
		{name: screenLeads, tables: []string{tableLeads, tableDismissed}, hasTables: true},
		{name: screenDraft, tables: []string{tableDraft}, hasTables: true},
		{name: screenApplied, tables: []string{allView, tableApplied, tableInterviewing, tableOffer, tableClosed}, hasTables: true},
		{name: screenStats, hasTables: false, render: model.statsView, key: model.statsKey, help: model.statsHelpKeys},
	}
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
