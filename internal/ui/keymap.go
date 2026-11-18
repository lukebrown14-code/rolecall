package ui

import (
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// keyBackspace is the named key the search input and command palette edit with.
const keyBackspace = "backspace"

// listAction is one row action the list and detail views accept: the keys that trigger
// it, how the help words it, its group and key-line priority, when it is listed and when
// it runs, and what it does. The help and the dispatcher both read this one table, so the
// two cannot drift apart.
//
// `keys` are the keys that run it; `keyLabel` is what the help shows (often a friendlier
// alias, e.g. "enter" for enter and v). `listed` is when the help advertises the key;
// `enabled` is when the dispatcher runs it. They are the same for almost every key. They
// differ for build, which stays quiet until the Owner has set the Plugin: the key still
// works and says what is missing, but the help does not advertise it. The Harness keys
// (assess fit, prepare docs, follow-up) are listed without a Harness too, dimmed in the
// help and palette with "(needs a Harness)", so the Owner can find them.
type listAction struct {
	keys     []string
	keyLabel string
	desc     func(model) string
	group    helpGroup
	prio     func(model) int
	listed   func(model) bool
	enabled  func(model) bool
	// early is true when the dispatcher returns at once instead of falling through to the
	// end-of-update sync: the actions that open a pop-up or start a run.
	early bool
	// alwaysInHelp keeps the key in the `?` pop-up while listed is false, so Help names it
	// before it first does something (u undo, before any move).
	alwaysInHelp bool
	// needsHarness marks a Handoff key: with no Harness set it is listed dimmed, help-only,
	// with needsHarnessNote after its description.
	needsHarness bool
	// why is what the key says when it is pressed where it does not apply ("x: only Leads
	// and Drafts can be dismissed"), or "" to stay silent; nil for keys that always apply.
	why func(model) string
	run func(m *model, key string) tea.Cmd
}

// needsHarnessNote follows a Handoff key's description while no Harness is set.
const needsHarnessNote = "(needs a Harness)"

// descConnectHarness is h's one name: in the help, the palette, the welcome checklist and
// the no-Harness error.
const descConnectHarness = "connect a Harness"

// onRowWhy is a why that speaks only when a row is under the cursor: with no row, a row
// key stays silent as before.
func onRowWhy(reason func(model) string) func(model) string {
	return func(m model) string {
		if !onRow(m) {
			return ""
		}
		return reason(m)
	}
}

// text is a fixed help description.
func text(s string) func(model) string { return func(model) string { return s } }

// fixed is a fixed key-line priority.
func fixed(prio int) func(model) int { return func(model) int { return prio } }

// always is an unconditional gate.
func always(model) bool { return true }

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

// searchJumps reports an empty view under a search that has matches on another view or
// screen: where enter goes to them.
func searchJumps(m model) bool {
	if m.detail != nil || !m.hasTables() || m.text == "" || onRow(m) {
		return false
	}
	_, j := m.searchElsewhere()
	return j.ok
}

// leadOrDraft reports whether the current row is still the Owner's to send.
func leadOrDraft(m model) bool {
	a, ok := m.current()
	return ok && a.Unsent()
}

// followUpRow reports whether the current row is Follow-up due.
func followUpRow(m model) bool {
	a, ok := m.current()
	return ok && followUpDue(a, m.now(), m.followUpDays)
}

// linkAction is a key that acts on the row's job-ad link: w opens it, c copies it.
func linkAction(key, desc string, run func(*model) tea.Cmd) listAction {
	return listAction{
		keys: []string{key}, keyLabel: key, desc: text(desc),
		group: groupAction, prio: fixed(prioMinor),
		listed:  hasLink,
		enabled: hasLink,
		why:     onRowWhy(func(model) string { return key + ": this Application has no link" }),
		run:     func(m *model, _ string) tea.Cmd { return run(m) },
	}
}

// listActions is every row action the list and detail views accept, in the order the help
// lists them. The dispatcher and the help both read it.
var listActions = []listAction{
	{
		keys: []string{"enter", "v"}, keyLabel: "enter",
		// One label at every width: "open" opens the detail, or the Document under the
		// cursor on the Docs tab.
		desc:  func(model) string { return descOpen },
		group: groupAction, prio: fixed(prioCore),
		listed: func(m model) bool {
			a, ok := m.current()
			if !ok {
				return false
			}
			return m.detail == nil || (m.detail.tab == tabDocs && a.Malformed == nil)
		},
		enabled: onRow,
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.selected(); ok {
				m.openDetail(a.Folder)
			}
			return nil
		},
	},
	{
		// A search that matches nothing here but something elsewhere: enter, which has no row to
		// open, goes to where the matches are.
		keys: []string{"enter"}, keyLabel: "enter",
		desc: func(m model) string {
			_, j := m.searchElsewhere()
			return "go to " + j.words
		},
		group: groupAction, prio: fixed(prioCore),
		listed:  searchJumps,
		enabled: searchJumps,
		run: func(m *model, _ string) tea.Cmd {
			if _, j := m.searchElsewhere(); j.ok {
				m.jumpToSearch(j)
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
			return descEdit
		},
		group: groupAction,
		prio: func(m model) int {
			switch a, ok := m.current(); {
			case ok && a.Malformed != nil:
				return prioCore
			case m.detail != nil:
				return prioDetailEdit
			}
			return prioMain
		},
		listed:  onRow,
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
		listed:  func(m model) bool { return onRow(m) && usable(m) },
		enabled: onRow,
		run: func(m *model, _ string) tea.Cmd {
			switch marked := m.markedHere(); {
			case len(marked) > 1:
				// Marked rows move together: the pop-up names the count, and the pick moves them.
				m.overlay = &moveMode{app: marked[0], options: bulkOptions(marked), bulk: marked}
				return nil
			case len(marked) == 1:
				// One marked row moves as a plain m on it would.
				a := marked[0]
				m.overlay = &moveMode{app: a, options: MoveOptions(a.Status, statusBeforeDismissed(a.StatusLog))}
				return nil
			}
			a, ok := m.current()
			if ok && a.Malformed != nil {
				// Refused at the key, as x is: the pop-up would offer Statuses for an
				// Application the store cannot read. The error offers e fix in editor.
				m.err = a.Malformed
				return nil
			}
			if ok {
				m.overlay = &moveMode{app: a, options: MoveOptions(a.Status, statusBeforeDismissed(a.StatusLog))}
			}
			return nil
		},
	},
	{
		// While a move can be undone u is core on the key line, so the toast's offer
		// outlasts the next key and a one-key dismiss always shows its way back.
		keys: []string{"u"}, keyLabel: "u", desc: text("undo"),
		group: groupAction, prio: fixed(prioCore), alwaysInHelp: true,
		// Undo acts on the last move, not the row under the cursor, so it stays listed on a
		// malformed row too.
		listed:  func(m model) bool { return len(m.undo) > 0 },
		enabled: always,
		run:     func(m *model, _ string) tea.Cmd { return m.undoLast() },
	},
	{
		// On the lean key line wherever it applies: triage is mostly x. In the detail view it
		// dismisses the open Lead or Draft and opens the next row, so reading then dismissing
		// is one key; marks are the list's.
		keys: []string{"x"}, keyLabel: "x", desc: text("dismiss"),
		group: groupAction, prio: fixed(prioLean),
		listed:  func(m model) bool { return (usable(m) && leadOrDraft(m)) || (m.detail == nil && m.markCount() > 0) },
		enabled: func(m model) bool { return leadOrDraft(m) || (m.detail == nil && m.markCount() > 0) },
		why:     onRowWhy(func(model) string { return "x: only Leads and Drafts can be dismissed · m to move" }),
		run: func(m *model, _ string) tea.Cmd {
			if m.detail == nil && m.markCount() > 0 {
				var cmd tea.Cmd
				m.overlay, cmd = m.bulkDismiss()
				return cmd
			}
			a, ok := m.current()
			switch {
			case !ok || !a.Unsent():
				return nil
			case m.detail != nil:
				return m.dismissFromDetail(a)
			}
			return m.move(a, store.StatusDismissed, false)
		},
	},
	{
		// Marks several rows for one x or m, and steps down so * * * marks three in a row.
		// * is free on every list; space, v and tab are not.
		keys: []string{keyMark}, keyLabel: keyMark,
		desc: func(m model) string {
			if a, ok := m.current(); ok && m.isMarked(a.Folder) {
				return "unmark row"
			}
			return "mark row"
		},
		group: groupAction, prio: fixed(prioMain),
		listed:  func(m model) bool { return m.detail == nil && onRow(m) && usable(m) },
		enabled: func(m model) bool { return m.detail == nil && onRow(m) && usable(m) },
		run: func(m *model, _ string) tea.Cmd {
			m.toggleMark()
			return nil
		},
	},
	{
		// Marks every row in the view. V (vim's whole-line select) is free on every list:
		// ctrl+a is GNU screen's prefix and a text field's start of line, and a is add.
		keys: []string{keyMarkAll}, keyLabel: keyMarkAll,
		desc: func(m model) string {
			if m.allMarked() {
				return "unmark all"
			}
			return "mark all"
		},
		group: groupAction, prio: fixed(prioMinor),
		listed:  func(m model) bool { return m.detail == nil && onRow(m) && usable(m) },
		enabled: func(m model) bool { return m.detail == nil && onRow(m) && usable(m) },
		run: func(m *model, _ string) tea.Cmd {
			m.markAll()
			return nil
		},
	},
	{
		keys: []string{"n"}, keyLabel: "n", desc: text(descNotes),
		group: groupAction,
		prio: func(m model) int {
			if m.detail != nil {
				return prioDetailNote
			}
			return prioMain
		},
		listed:  func(m model) bool { return onRow(m) && usable(m) },
		enabled: onRow,
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
		listed: func(m model) bool {
			a, ok := m.current()
			return ok && a.Malformed == nil && m.plugin != "" && buildable(a)
		},
		enabled: always,
		run:     func(m *model, _ string) tea.Cmd { return m.startBuild() },
	},
	linkAction("w", "open link", (*model).openLink),
	linkAction("c", "copy link", (*model).copyLink),
	{
		keys: []string{"d"}, keyLabel: "d", desc: text("download"),
		group: groupAction, prio: fixed(prioMinor),
		listed: func(m model) bool {
			a, ok := m.current()
			return ok && a.Malformed == nil && len(a.PDFs) > 0
		},
		enabled: func(m model) bool {
			a, ok := m.current()
			return ok && len(a.PDFs) > 0
		},
		why: onRowWhy(func(model) string { return "d: no PDFs built for this Application yet" }),
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok && len(a.PDFs) > 0 {
				return m.download(a)
			}
			return nil
		},
	},
	{
		keys: []string{"f"}, keyLabel: "f", desc: text("assess fit"),
		group: groupAction, prio: fixed(prioMinor), needsHarness: true,
		listed:  onRow,
		enabled: always,
		run:     func(m *model, _ string) tea.Cmd { return m.startHandoff(skillAssessFit) },
	},
	{
		// Only on a Follow-up due Application. It has its own key so r is always rescan.
		keys: []string{"F"}, keyLabel: "F", desc: text(descNudge),
		group: groupAction, prio: fixed(prioMinor), needsHarness: true,
		listed:  func(m model) bool { return followUpRow(m) && usable(m) },
		enabled: followUpRow,
		why:     onRowWhy(func(model) string { return "F: only on a Follow-up due Application" }),
		run:     func(m *model, _ string) tea.Cmd { return m.startHandoff(skillDraftFollowUp) },
	},
	{
		// Records today's follow-up without a Status change, so the ⚑ clears (followup.go).
		keys: []string{keyFollowedUp}, keyLabel: keyFollowedUp, desc: text(descFollowedUp),
		group: groupAction, prio: fixed(prioMinor),
		listed:  func(m model) bool { a, ok := m.current(); return ok && usable(m) && followUpStatus(a.Status) },
		enabled: func(m model) bool { a, ok := m.current(); return ok && a.Malformed == nil && followUpStatus(a.Status) },
		why:     onRowWhy(func(model) string { return keyFollowedUp + ": only on Applied and Interviewing Applications" }),
		run: func(m *model, _ string) tea.Cmd {
			if a, ok := m.current(); ok {
				return m.recordFollowUp(a)
			}
			return nil
		},
	},
	{
		keys: []string{"p"}, keyLabel: "p", desc: text("prepare docs"),
		group: groupAction, prio: fixed(prioMinor), needsHarness: true,
		listed: func(m model) bool { return usable(m) && leadOrDraft(m) },
		// Without a Harness it runs anywhere, to say so; with one, only where it applies.
		enabled: func(m model) bool { return m.harness == "" || !onRow(m) || leadOrDraft(m) },
		why:     onRowWhy(func(model) string { return "p: only Leads and Drafts get documents prepared" }),
		run:     func(m *model, _ string) tea.Cmd { return m.startHandoff(skillPrepareDocuments) },
	},
	{
		// "view", not "status": s changes which rows show, while m changes a Status.
		keys: []string{"s"}, keyLabel: "s", desc: text(descChangeView),
		group: groupFilter, prio: fixed(prioMinor),
		listed:  func(m model) bool { return m.detail == nil && len(m.views()) > 1 },
		enabled: func(m model) bool { return m.detail == nil && len(m.views()) > 1 },
		why: func(m model) string {
			if m.detail != nil {
				return "s: changes the view from the list only"
			}
			return "" // one view: nothing to change to
		},
		run: func(m *model, _ string) tea.Cmd {
			// The same cycle as ←→ and as t on the Track filter: All, each Status, Closed, All.
			m.table = (m.table + 1) % len(m.views())
			m.cursor = 0
			return nil
		},
	},
	{
		keys: []string{"o"}, keyLabel: "o",
		desc: func(m model) string {
			if m.leadsNewest {
				return "sort by deadline"
			}
			return "sort newest"
		},
		group: groupFilter, prio: fixed(prioMinor),
		listed:  func(m model) bool { return m.detail == nil && m.views()[m.table] == tableLeads && !m.welcoming() },
		enabled: func(m model) bool { return m.detail == nil && m.views()[m.table] == tableLeads },
		why:     func(model) string { return "o: sorts the Leads view only" },
		run: func(m *model, _ string) tea.Cmd {
			folder := m.cursorFolder()
			m.leadsNewest = !m.leadsNewest
			m.keepCursor(folder)
			m.toast = "Leads by deadline, soonest first"
			if m.leadsNewest {
				m.toast = "Leads newest first"
			}
			return nil
		},
	},
	{
		// z (vim's view-adjusting prefix, free on every list) switches one-line rows at
		// wide sizes, where one line has room for every column.
		keys: []string{"z"}, keyLabel: "z",
		desc: func(m model) string {
			if m.compactRows {
				return "two-line rows"
			}
			return "compact rows"
		},
		group: groupFilter, prio: fixed(prioMinor),
		listed:  func(m model) bool { return m.detail == nil && m.layout().wide() && !m.welcoming() },
		enabled: func(m model) bool { return m.detail == nil && m.layout().wide() },
		run:     func(m *model, _ string) tea.Cmd { return m.toggleCompactRows() },
	},
	{
		keys: []string{"a"}, keyLabel: "a", desc: text("add"),
		group: groupAction, prio: fixed(prioCore),
		listed:  func(m model) bool { return m.detail == nil },
		enabled: always,
		run:     func(m *model, _ string) tea.Cmd { return m.openAdd() },
	},
	{
		keys: []string{"/"}, keyLabel: "/", desc: text("search"),
		group: groupFilter, prio: fixed(prioCore),
		listed:  func(m model) bool { return m.detail == nil && !m.welcoming() }, // nothing to search yet
		enabled: always,
		run: func(m *model, _ string) tea.Cmd {
			m.openSearch()
			return nil
		},
	},
	{
		// One key undoes every filter: the search, the Track and the view. backspace is free
		// on every list; the search input has its own.
		keys: []string{keyBackspace}, keyLabel: keyBackspace, desc: text(descClearFilters),
		group: groupFilter, prio: fixed(prioMinor),
		listed:  func(m model) bool { return m.detail == nil && m.anyFilter() },
		enabled: func(m model) bool { return m.detail == nil },
		run: func(m *model, _ string) tea.Cmd {
			m.clearFilters()
			return nil
		},
	},
}

// descClearFilters is backspace's one name, in the help, the palette and the hint esc
// leaves when a Track or view is still on.
const descClearFilters = "clear all filters"

// anyFilter reports whether the list shows less than its screen's first view would: a
// search, a Track or another view is on.
func (m model) anyFilter() bool { return m.text != "" || m.track > 0 || m.table > 0 }

// clearFilters is backspace on a list: the search, the Track and the view go back to
// none, All and the screen's first view, and the toast names what went.
func (m *model) clearFilters() {
	var gone []string
	if m.text != "" {
		gone = append(gone, `search "/`+m.text+`"`)
	}
	if m.track > 0 {
		gone = append(gone, "Track "+m.trackName())
	}
	if m.table > 0 {
		gone = append(gone, "view "+m.views()[m.table])
	}
	if len(gone) == 0 {
		m.toast, m.toastKind = "No filters on", toastNote
		return
	}
	m.text, m.track, m.table, m.cursor = "", 0, 0, 0
	m.toast = "Cleared " + strings.Join(gone, ", ")
}

// descChangeView is s's one name, in the help, the palette, the space menu and an empty
// view's hint. ←→ do the same; s is the key the menu and palette can send.
const descChangeView = "change view"

// actionKeys is the row actions the help lists here, in order, built from the one table.
// A Handoff key with no Harness set is help-only and dimmed, marked needsHarnessNote.
func (m model) actionKeys() []helpKey {
	var keys []helpKey
	for _, a := range listActions {
		if !a.listed(m) {
			continue
		}
		if a.needsHarness && m.harness == "" {
			k := help(a.group, a.keyLabel, a.desc(m)+" "+needsHarnessNote).at(prioHelpOnly)
			k.dim = true
			keys = append(keys, k)
			continue
		}
		keys = append(keys, help(a.group, a.keyLabel, a.desc(m)).at(a.prio(m)))
	}
	return keys
}

// dormantKeys is the row actions the `?` pop-up names although they do nothing yet here,
// help-only so they never reach the key line.
func (m model) dormantKeys() []helpKey {
	if !m.hasTables() {
		return nil
	}
	var keys []helpKey
	for _, a := range listActions {
		if a.alwaysInHelp && !a.listed(m) && usable(m) {
			keys = append(keys, help(a.group, a.keyLabel, a.desc(m)).at(prioHelpOnly))
		}
	}
	return keys
}

// runListAction finds the action a key triggers here: the first enabled action whose keys
// include it. A known key that does not apply here says why in a note toast instead of
// doing nothing. handled is false when no action claims the key. early is true when the
// caller should return at once rather than fall through to the end-of-update sync.
func (m *model) runListAction(key string) (cmd tea.Cmd, handled, early bool) {
	for _, a := range listActions {
		if a.enabled(*m) && slices.Contains(a.keys, key) {
			return a.run(m, key), true, a.early
		}
	}
	for _, a := range listActions {
		if a.why == nil || !slices.Contains(a.keys, key) {
			continue
		}
		if why := a.why(*m); why != "" {
			m.toast, m.toastKind = why, toastNote
			return nil, true, false
		}
	}
	return nil, false, false
}

// connectsHarness reports whether h opens Settings › This machine on the Harness row
// here: while no Harness is set, on every screen but Settings, after any Application has
// been added as well as on the welcome. It is how the no-Harness error is fixed.
func (m model) connectsHarness(key string) bool {
	return key == "h" && m.harness == "" && m.screen != settingsScreen && !m.fieldOpen()
}
