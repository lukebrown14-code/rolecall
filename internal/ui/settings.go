package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// settingsFile is the Workspace file the Search, Documents and Tracking groups write.
const settingsFile = "workspace.json"

// configFile is the machine file This machine and follow-up days are kept in.
const configFile = "config.toml"

// Settings screen sizing.
const (
	// maxLabelWidth caps the label column, which is otherwise as wide as the group's longest label.
	maxLabelWidth = 24
	// maxFieldWidth is the widest an inline field grows: a value is edited where it sits.
	maxFieldWidth = 30
	// minFieldWidth is the narrowest an inline field is drawn.
	minFieldWidth = 12
)

// editorExamples is the help line for the Editor setting: values for editor in config.toml.
// GUI Editors need their wait flag, or rolecall rescans before editing finishes.
const editorExamples = "Editor examples: nvim, code --wait, zed --wait, hx, subl --wait"

// settingsBack is where Esc returns to after `,` opened Settings.
type settingsBack struct {
	screen, table, cursor int
	detail                *detailState
}

// openSettings switches to Settings from wherever the Owner is, remembering the screen,
// table, cursor and open detail so Esc can restore them. The filter lives on the model and
// is untouched.
func (m model) openSettings() model {
	m.back = &settingsBack{m.screen, m.table, m.cursor, m.detail}
	m.screen, m.table, m.cursor, m.detail = settingsScreen, 0, 0, nil
	return m.loadSettings()
}

// closeSettings is Esc after openSettings.
func (m model) closeSettings() model {
	b := m.back
	m.back = nil
	m.screen, m.table, m.cursor, m.detail = b.screen, b.table, b.cursor, b.detail
	return m
}

// settingsUndo is the last Settings save, so u can put the old value back once.
type settingsUndo struct {
	what    string // "Editor back to vi"
	restore func(model) model
}

// loadSettings re-reads workspace.json and config.toml. It runs on entering the screen, on r and after
// the editor closes; the group, row, focus and undo are kept.
func (m model) loadSettings() model {
	st := settingsState{}
	if m.settings != nil {
		st = *m.settings
	}
	st, err := st.Load(m.store)
	if err != nil {
		m.err = err
		return m
	}
	if m.configHome != "" {
		if m = m.loadMachine(); m.err != nil {
			return m
		}
	}

	m.settings = &st
	st.Row = m.selectableRow(st.Group, st.Row, 1)
	return m
}

// settingsGroupHandler is one Settings group's keys: how the help describes them and how
// the screen dispatches them. The screen reads one table, so a group cannot be dispatched
// without also being described, and the group order is data rather than a chain of calls.
type settingsGroupHandler struct {
	group int
	help  func(model) []helpKey
	// key handles the group's row keys; nil when the group has none of its own (This
	// machine's rows activate through Enter, like every other row).
	key func(model, tea.KeyPressMsg) (model, bool)
}

// settingsHandlers is every group's handler, in display order.
var settingsHandlers = []settingsGroupHandler{
	{group: settings.GroupSearch, help: model.searchHelpKeys, key: model.searchKey},
	{group: settings.GroupDocuments, help: model.gateHelpKeys, key: model.gatesKey},
	{group: settings.GroupTracking, help: model.trackHelpKeys, key: model.tracksKey},
	{group: settings.GroupMachine, help: model.machineHelpKeys},
}

// settingsHelpKeys is the keys Settings accepts. Application actions are absent, and so
// are the global keys.
func (m model) settingsHelpKeys() []helpKey {
	if m.settings != nil && m.settings.Edit != nil {
		return []helpKey{help(groupAction, "enter", "set").at(prioCore), help(groupNav, "esc", "cancel").at(prioPinned)}
	}
	keys := []helpKey{}
	for _, h := range settingsHandlers {
		keys = append(keys, h.help(m)...)
	}
	if m.settings != nil && m.settings.Undo != nil {
		keys = append(keys, help(groupAction, "u", "undo").at(prioMinor))
	}
	if m.settings != nil && settingsGroups[m.settings.Group] == groupMachineName {
		if m.configHome != "" {
			keys = append(keys, help(groupAction, "e", "edit "+configFile))
		}
	} else {
		keys = append(keys, help(groupAction, "e", "edit "+settingsFile))
	}
	if m.settings != nil && m.settings.Ws.Err == nil {
		keys = append(keys, help(groupAction, "ctrl+s", "save").at(prioMain), help(groupAction, "ctrl+z", "revert").at(prioMinor),
			help(groupNav, "[ ]", "section").at(prioMain), help(groupNav, "j k", "move"))
	}
	if m.err != nil {
		keys = append(keys, help(groupNav, "esc", "clear error").at(prioPinned))
	} else if m.back != nil {
		keys = append(keys, help(groupNav, "esc", "back").at(prioPinned))
	}
	return keys
}

// settingsKey handles the keys Settings accepts. e hands the group's file to the Owner's
// editor and the screen re-reads when it exits; every other write goes through a pop-up.
func (m model) settingsKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if next, ok := m.renameKey(k); ok {
		return next, nil
	}
	switch k.String() {
	case "r":
		m.rescanAll()
		return m.loadSettings(), nil
	case "?":
		m.overlay = &helpMode{}
		return m, nil
	case "q":
		return m, m.quit()
	case "e":
		// On This machine e opens config.toml (Editor and the rest); elsewhere workspace.json.
		if m.settings != nil && settingsGroups[m.settings.Group] == groupMachineName {
			if m.configHome == "" {
				return m, nil
			}
			return m, m.editConfig()
		}
		return m, m.editFile(m.store.Root(), settingsFile)
	case "esc":
		if m.back != nil {
			return m.leaveSettings(func(m model) model { return m.closeSettings() }), nil
		}
	}
	if m.settings == nil || m.settings.Ws.Err != nil {
		return m, nil // an invalid file offers e and nothing else
	}
	switch k.String() {
	case "ctrl+s":
		return m.saveSettings(), nil
	case "ctrl+z":
		return m.revertSettings(), nil
	}
	if k.String() == "u" && m.settings.Undo != nil {
		return m.undoSetting(), nil
	}
	for _, h := range settingsHandlers {
		if h.key == nil {
			continue
		}
		if next, ok := h.key(m, k); ok {
			return next, nil
		}
	}
	st := *m.settings
	switch k.String() {
	case "enter":
		if !st.Header {
			return m.activateSetting()
		}
	default:
		st = st.Navigate(k.String(), m.settingsMachine())
	}
	m.settings = &st
	return m, nil
}

// saved reports a Settings save on the feedback line, naming the file it went to, and
// keeps restore so u can put the old value back.
func (m model) saved(msg, file string, undo *settingsUndo) model {
	m.toast = msg + " · " + file
	if m.settings != nil {
		st := *m.settings
		st.Undo = undo
		m.settings = &st
	}
	return m
}

// undoSetting puts back the value the last Settings save replaced.
func (m model) undoSetting() model {
	u := m.settings.Undo
	m = u.restore(m)
	st := *m.settings
	st.Undo = nil
	m.settings = &st
	if m.err == nil {
		m.toast, m.toastHint = "Undone · "+u.what, ""
	}
	return m
}

// settingsView is the Settings screen in the same frame Profile uses.
func (m model) settingsView(p pal) string {
	w, h := m.viewWidth(), m.profileBodyHeight()
	return m.frame(p, []string{m.settingsContextRow(p, w)}, m.settingsBody(p, w, h), 0, focusNone, m.statusRow(p, w, m.keyLine(p, w)))
}

// settingsTabs is the sections as `[ Tracking ]  Tracks …`: the active one in the single
// selection style, brackets and all, so it reads without colour. A narrow row keeps only
// the active section and its place.
func (m model) settingsTabs(p pal, w int) string {
	active := func(name string) string { return p.SelText(true).Render("[ " + name + " ]") }
	var parts []string
	for i, name := range settingsGroups {
		if i == m.settings.Group {
			parts = append(parts, active(name))
		} else {
			parts = append(parts, p.Text(p.Dim).Render(name))
		}
	}
	full := strings.Join(parts, "  ")
	if w >= settingsTabsMinWidth {
		return full
	}
	return active(settingsGroups[m.settings.Group]) + p.Text(p.Dim).Render(" "+strconv.Itoa(m.settings.Group+1)+"/"+strconv.Itoa(len(settingsGroups)))
}

// settingsTabsMinWidth is the width from which every section is named.
const settingsTabsMinWidth = 84

// settingsContextRow is the heading and the section tabs, with the unsaved-change count on
// the right. The cursor rests here too (k from the first field): ctrl+z on it reverts all.
func (m model) settingsContextRow(p pal, w int) string {
	lead := strings.Repeat(" ", inset)
	if m.settings == nil || m.settings.Ws.Err != nil {
		return strings.Repeat(" ", inset) + p.Heading("SETTINGS")
	}
	if m.settings.Header {
		lead = strings.Repeat(" ", edge) + p.SelGutter(true)
	}
	left := lead + p.Heading("SETTINGS") + "  " + m.settingsTabs(p, w)
	right := ""
	if n := m.settings.Dirty(); n > 0 {
		right = p.Text(p.Warn).Bold(true).Render("● " + plural(n, "unsaved change", "unsaved changes"))
	}
	return contextRow(left, m.withChip(p, right), w)
}

// settingsBody is the settings of the selected section, or the invalid-file notice.
func (m model) settingsBody(p pal, w, h int) string {
	pad := strings.Repeat(" ", inset)
	if m.settings != nil && m.settings.Ws.Err != nil {
		head := "workspace.json is not valid JSON"
		if m.settings.Ws.NewerSchema {
			head = "workspace.json is schema " + strconv.Itoa(m.settings.Ws.Schema) + ", from a newer rolecall"
		}
		text := pad + p.Text(p.Bad).Render(head) + "\n" + pad +
			p.Text(p.Fg).Render(m.settings.Ws.Path) + "\n" + pad + p.Text(p.Fg).Render(m.settings.Ws.Err.Error()) +
			"\n\n" + pad + p.Text(p.Dim).Render("Press e to fix it in your editor. Nothing is written.")
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, text)
	}
	if m.settings == nil {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, "")
	}
	return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).PaddingLeft(edge).
		Render(m.settingsRowsView(p, w-edge-rightMargin, h))
}

// settingsRowsView lists the selected group's settings, one per line, in three columns:
// label, value, and where the value comes from. The row marker shows only while the
// settings pane has focus.
func (m model) settingsRowsView(p pal, width, h int) string {
	rows := m.settingsRows(m.settings.Group)
	valueW, labelW, tailW := 0, 0, 0
	for _, r := range rows {
		tailW = max(tailW, xansi.StringWidth(r.Tail()))
		if !r.Action {
			labelW = max(labelW, profileMarkerWidth+xansi.StringWidth(r.Label)+gutter)
		}
		w := xansi.StringWidth(r.ValueText())
		if r.Value == "" && !r.Action {
			w = xansi.StringWidth("not set")
		}
		valueW = max(valueW, w)
	}
	labelW = min(labelW, maxLabelWidth)
	room := width - labelW
	valueW = min(valueW, max(minValueRoom, room-tailW-gutter))
	marked := !m.settings.Header
	editing := m.settings.Edit
	var lines []string
	selLine := 0
	for i, r := range rows {
		if r.Section != "" {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, strings.Repeat(" ", profileMarkerWidth)+p.Heading(r.Section))
		}
		selected := marked && i == m.settings.Row
		set, isSetting := m.rowSetting(m.settings.Group, i, len(rows))
		if isSetting {
			if _, dirty := m.settings.Pending[set.key]; dirty {
				r.Was = m.savedText(set)
				if r.Was == "" {
					r.Was = "not set"
				}
			}
		}
		if i == m.settings.Row {
			selLine = len(lines)
		}
		mark, label := "  ", p.Text(p.Dim).Render(r.Label)
		switch {
		case selected:
			mark, label = p.SelGutter(true), p.SelText(true).Render(r.Label)
		case r.Action:
			label = p.Text(p.Sub).Render(r.Label)
		}
		line := mark + label
		if r.Action {
			lines = append(lines, xansi.Truncate(line, width, "…"))
			continue
		}
		bg := p.SelBase(selected)
		pad := bg.Render(strings.Repeat(" ", max(1, labelW-profileMarkerWidth-xansi.StringWidth(r.Label))))
		var value string
		if r.Value == "" {
			value = bg.Foreground(p.Dim).Italic(true).Render("not set")
		} else {
			value = bg.Foreground(p.Fg).Render(fitValue(r.Label, r.ValueText(), valueW))
		}
		value += bg.Render(strings.Repeat(" ", max(0, valueW-xansi.StringWidth(value))))
		tail := bg.Foreground(p.Dim).Render(r.Note())
		if editing != nil && isSetting && editing.Key == set.key {
			// The field opens where the value sits, narrow, with the end of what is typed.
			fw := min(maxFieldWidth, max(minFieldWidth, valueW))
			shown := xansi.TruncateLeft(editing.Buf, max(0, xansi.StringWidth(editing.Buf)-(fw-1)), "…")
			value = bg.Foreground(p.Fg).Underline(true).Render(shown) + bg.Foreground(p.Accent).Render("▏")
			value += bg.Render(strings.Repeat(" ", max(0, fw-xansi.StringWidth(shown)-1)))
			tail = bg.Foreground(p.Dim).Render(r.Unit)
			if editing.Err != "" {
				tail = bg.Foreground(p.Bad).Render(errorMark + " " + editing.Err)
			}
		}
		if r.Warning != "" {
			tail += bg.Foreground(p.Warn).Render(warnSep + r.Warning)
		}
		fillLine := func(s string) string {
			s = xansi.Truncate(s, width, "…")
			if selected {
				return p.SelLine(true, width, s)
			}
			return s
		}
		line += pad + value
		if r.Tail() != "" {
			// The source and any warning are never cut: without room they get their own line.
			if width-labelW-valueW-gutter >= xansi.StringWidth(r.Tail()) {
				line += bg.Render(strings.Repeat(" ", gutter)) + tail
				lines = append(lines, fillLine(line))
			} else {
				lines = append(lines, fillLine(line),
					xansi.Truncate(strings.Repeat(" ", labelW)+tail, width, "…"))
			}
			continue
		}
		lines = append(lines, fillLine(line))
	}
	if settingsGroups[m.settings.Group] == groupMachineName {
		examples := wrapWords(editorExamples, max(1, width-profileMarkerWidth))
		if len(lines)+1+len(examples) <= h {
			lines = append(lines, "") // a short terminal loses this spacer before the examples
		}
		for _, l := range examples {
			lines = append(lines, strings.Repeat(" ", profileMarkerWidth)+p.Text(p.Dim).Render(l))
		}
	}
	if note := m.deferredNote(); note != "" && len(lines)+2 <= h {
		// Short terminals lose this line before they lose a field.
		lines = append(lines, "", strings.Repeat(" ", profileMarkerWidth)+p.Text(p.Dim).Render(note))
	}
	return strings.Join(p.ClipLines(lines, selLine-h/bothSides, h, strings.Repeat(" ", profileMarkerWidth)), "\n")
}

// minValueRoom is the least width worth giving a value.
const minValueRoom = 12

// fitValue cuts a value that does not fit. A path keeps its end, where the folder names
// that tell Workspaces apart are; other values keep their start.
func fitValue(label, value string, room int) string {
	if xansi.StringWidth(value) <= room || room < 2 {
		return value
	}
	switch label {
	case labelWorkspace, labelDownloads, labelPlugin:
		return "…" + xansi.TruncateLeft(value, xansi.StringWidth(value)-room+1, "")
	}
	return xansi.Truncate(value, room, "…")
}

type settingRow = settings.Row
type settingsState = settings.State[settingsUndo]

const warnSep = settings.WarnSep

var settingsGroups = settings.Groups

const (
	groupMachineName = settings.GroupMachineName
	labelEditor      = settings.LabelEditor
	labelFollowUp    = settings.LabelFollowUp
	labelHarness     = settings.LabelHarness
	labelWorkspace   = settings.LabelWorkspace
	labelDownloads   = settings.LabelDownloads
	labelPlugin      = settings.LabelPlugin
	labelASCII       = settings.LabelASCII
	labelInherit     = settings.LabelInherit
)

// settingsMachine is the machine settings as the screen shows them: the saved values with
// any unsaved edit laid over them.
func (m model) settingsMachine() settings.Machine {
	mc := settings.Machine{Workspace: m.store.Root(), EditorSource: m.editorSource, Downloads: m.downloads, Harness: m.harness,
		Plugin: m.plugin, ASCII: m.ascii, Inherit: m.inherit, FollowUpDays: m.followUpDays, Sources: m.sources}
	editor := m.editor
	if m.settings != nil {
		pend := m.settings.Pending
		if v, ok := pend[keyEditor]; ok {
			editor = v
		}
		if v, ok := pend["downloads"]; ok {
			mc.Downloads = v
		}
		if v, ok := pend["harness"]; ok {
			mc.Harness = v
		}
		if v, ok := pend["plugin"]; ok {
			mc.Plugin = v
		}
		if v, ok := pend["ascii"]; ok {
			mc.ASCII = v == yesNo(true)
		}
		if v, ok := pend[keyInherit]; ok {
			mc.Inherit = v == yesNo(true)
		}
		if v, ok := pend[keyFollowUpDays]; ok {
			mc.FollowUpDays, _ = strconv.Atoi(v)
		}
	}
	mc.Editor = ResolveEditor(editor)
	mc.EditorAvailable = editorOnPath(mc.Editor)
	return mc
}
func (m model) settingsRows(group int) []settingRow {
	st := *m.settings
	st.Breakdown = m.trackBreakdown(st)
	return st.Rows(group, m.settingsMachine())
}

// trackBreakdown explains each Track's count so the numbers add up: how many of its
// Applications are active and how many closed (Rejected, Withdrawn, Dismissed).
func (m model) trackBreakdown(st settingsState) map[string]string {
	closed := map[string]int{}
	for _, a := range m.apps {
		switch a.Status {
		case store.StatusRejected, store.StatusWithdrawn, store.StatusDismissed:
			closed[a.Track]++
		}
	}
	out := map[string]string{}
	for _, t := range st.Ws.Tracks {
		c := min(closed[t], st.TrackUse[t])
		out[t] = " · " + strconv.Itoa(st.TrackUse[t]-c) + " active"
		if c > 0 {
			out[t] += " · " + strconv.Itoa(c) + " closed"
		}
	}
	return out
}
func (m model) trackingRows() []settingRow { return m.settingsRows(settings.GroupTracking) }
func (m model) selectableRow(group, row, dir int) int {
	return settings.SelectableRow(m.settingsRows(group), row, dir)
}
