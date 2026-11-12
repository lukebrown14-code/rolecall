package ui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// listAction is one row action the list and detail views accept: the keys that trigger
// it, how the help words it, its group and key-line priority, when it applies, and what
// it does. The help and the dispatcher both read this one table, so what a key does and
// what the help claims can never drift apart.
//
// `keys` are the keys that run it; `keyLabel` is what the help shows (often a friendlier
// alias, e.g. "enter" for enter and v). `enabled` is the single gate: it decides both
// whether the help lists the action and whether the dispatcher runs it.
type listAction struct {
	keys     []string
	keyLabel string
	desc     func(model) string
	group    helpGroup
	prio     func(model) int
	enabled  func(model) bool
	run      func(m *model, key string) tea.Cmd
}

// text is a fixed help description.
func text(s string) func(model) string { return func(model) string { return s } }

// fixed is a fixed key-line priority.
func fixed(prio int) func(model) int { return func(model) int { return prio } }

// onRow reports whether a row is under the cursor (or the detail view's Application is
// open).
func onRow(m model) bool { _, ok := m.current(); return ok }

// usable reports whether the current row can be acted on: no row, or a row that parsed.
// A malformed row can be inspected and fixed in the Editor; Move, undo, notes, build and
// downloads are refused there.
func usable(m model) bool {
	a, ok := m.current()
	return !ok || a.Malformed == nil
}

// sent reports whether the current row is a sent Application (Applied or Interviewing),
// which the follow-up and Handoff rules single out.
func sent(m model) bool {
	a, ok := m.current()
	return ok && (a.Status == store.StatusApplied || a.Status == store.StatusInterviewing)
}

// leadOrDraft reports whether the current row is still the Owner's to send.
func leadOrDraft(m model) bool {
	a, ok := m.current()
	return ok && (a.Status == store.StatusLead || a.Status == store.StatusDraft)
}

// listActions is every row action the list and detail views accept, in the order the help
// lists them. The dispatcher and the help both read it.
var listActions = []listAction{
	{
		keys: []string{"enter", "v"}, keyLabel: "enter", desc: text("open"),
		group: groupAction, prio: fixed(prioCore),
		enabled: func(m model) bool {
			a, ok := m.current()
			if !ok {
				return false
			}
			return m.detail == nil || (m.detail.tab == tabDocs && a.Malformed == nil)
		},
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.selected(); ok {
				m.openDetail(a.Folder)
			}
			return nil
		},
	},
	{
		keys: []string{"e"}, keyLabel: "e",
		desc: func(m model) string {
			if a, ok := m.current(); ok && a.Malformed != nil {
				return "fix in editor"
			}
			return "edit"
		},
		group: groupAction,
		prio: func(m model) int {
			if a, ok := m.current(); ok && a.Malformed != nil {
				return prioCore
			}
			return prioMain
		},
		enabled: onRow,
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok {
				return m.openEdit(a)
			}
			return nil
		},
	},
	{
		keys: []string{"m"}, keyLabel: "m", desc: text("move"),
		group: groupAction, prio: fixed(prioCore),
		enabled: func(m model) bool { return onRow(m) && usable(m) },
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok {
				m.overlay = &moveMode{app: a, options: MoveOptions(a.Status, statusBeforeDismissed(a.StatusLog))}
			}
			return nil
		},
	},
	{
		keys: []string{"u"}, keyLabel: "u", desc: text("undo"),
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool { return usable(m) && len(m.undo) > 0 },
		run: func(m *model, _ string) tea.Cmd {
			if len(m.undo) == 0 {
				return nil
			}
			last := m.undo[len(m.undo)-1]
			for _, a := range m.apps {
				if a.Folder == last.folder {
					cmd := m.move(a, last.status, true)
					if m.err == nil {
						m.undo = m.undo[:len(m.undo)-1]
					}
					return cmd
				}
			}
			return nil
		},
	},
	{
		keys: []string{"x"}, keyLabel: "x", desc: text("dismiss"),
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool { return m.detail == nil && leadOrDraft(m) && usable(m) },
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok && (a.Status == store.StatusLead || a.Status == store.StatusDraft) {
				return m.move(a, store.StatusDismissed, false)
			}
			return nil
		},
	},
	{
		keys: []string{"n"}, keyLabel: "n", desc: text("notes"),
		group: groupAction,
		prio: func(m model) int {
			if m.detail != nil {
				return prioMinor // the reader's navigation outlasts it
			}
			return prioMain
		},
		enabled: func(m model) bool { return onRow(m) && usable(m) },
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok {
				return m.editNotes(a.Folder)
			}
			return nil
		},
	},
	{
		keys: []string{"b"}, keyLabel: "b", desc: text("build"),
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool {
			a, ok := m.current()
			return ok && a.Malformed == nil && m.plugin != "" && buildable(a)
		},
		run: func(m *model, _ string) tea.Cmd { return m.startBuild() },
	},
	{
		keys: []string{"d"}, keyLabel: "d", desc: text("download"),
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool {
			a, ok := m.current()
			return ok && a.Malformed == nil && len(a.PDFs) > 0
		},
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok && len(a.PDFs) > 0 {
				return m.download(a)
			}
			return nil
		},
	},
	{
		keys: []string{"f"}, keyLabel: "f", desc: text("assess fit"),
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool { return onRow(m) && m.harness != "" },
		run:     func(m *model, _ string) tea.Cmd { return m.startHandoff(skillAssessFit) },
	},
	{
		keys: []string{"p"}, keyLabel: "p", desc: text("prepare docs"),
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool { return usable(m) && leadOrDraft(m) && m.harness != "" },
		run:     func(m *model, _ string) tea.Cmd { return m.startHandoff(skillPrepareDocuments) },
	},
	{
		keys: []string{"s"}, keyLabel: "s",
		desc: func(m model) string {
			if m.leadsNewest {
				return "sort by close date"
			}
			return "sort newest"
		},
		group: groupAction, prio: fixed(prioMinor),
		enabled: func(m model) bool { return m.detail == nil && m.views()[m.table] == tableLeads },
		run: func(m *model, _ string) tea.Cmd {
			folder := m.cursorFolder()
			m.leadsNewest = !m.leadsNewest
			m.keepCursor(folder)
			m.toast = "Leads by close date, soonest first"
			if m.leadsNewest {
				m.toast = "Leads newest first"
			}
			return nil
		},
	},
	{
		keys: []string{"a"}, keyLabel: "a", desc: text("add"),
		group: groupAction, prio: fixed(prioCore),
		enabled: func(m model) bool { return m.detail == nil },
		run:     func(m *model, _ string) tea.Cmd { return m.openAdd() },
	},
	{
		keys: []string{"/"}, keyLabel: "/", desc: text("search"),
		group: groupAction, prio: fixed(prioCore),
		enabled: func(m model) bool { return m.detail == nil },
		run: func(m *model, _ string) tea.Cmd {
			m.overlay = &searchMode{}
			return nil
		},
	},
}

// actionKeys is the row actions that apply here, in help order, built from the one table.
func (m model) actionKeys() []helpKey {
	var keys []helpKey
	for _, a := range listActions {
		if a.enabled(m) {
			keys = append(keys, help(a.group, a.keyLabel, a.desc(m)).at(a.prio(m)))
		}
	}
	return keys
}
