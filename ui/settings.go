package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/store"
)

// settingsFile is the one file the Settings screen opens with e.
const settingsFile = "workspace.json"

// The two panes tab moves between.
const (
	settingsList = iota
	settingsRows
	settingsPanes
)

// Settings screen sizing mirrors Profile: a fixed list column, then the settings.
const (
	settingsSplitWidth = profileSplitWidth
	settingsListWidth  = 20
	settingsLabelWidth = 24
)

// settingsGroups are the groups in display order.
var settingsGroups = []string{"Search", "Documents", "Tracking", groupMachineName}

const groupMachineName = "This machine"

// Labels of the machine settings, which carry a source.
const (
	labelEditor    = "Editor"
	labelFollowUp  = "Follow-up days"
	labelHarness   = "Harness"
	labelWorkspace = "Workspace"
	labelDownloads = "Downloads"
	labelPlugin    = "Plugin"
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

// sourceParts is what follows a setting's value: where it came from and, for the Editor,
// a warning when its executable is not on PATH. Neither is ever cut to fit.
func (m model) sourceParts(r settingRow) []string {
	if r.value == "" || r.value == "not set" {
		return nil
	}
	src := ""
	switch r.label {
	case labelEditor:
		src = m.editorSource
	case labelWorkspace:
		src = m.sources.Workspace
	case labelFollowUp:
		src = m.sources.FollowUpDays
	case labelDownloads:
		src = m.sources.Downloads
	case labelPlugin:
		src = m.sources.Plugin
	case labelHarness:
		src = m.sources.Harness
	}
	var parts []string
	if src != "" {
		parts = append(parts, "("+src+")")
	}
	if r.label == labelEditor && !editorOnPath(r.value) {
		parts = append(parts, "(not found on PATH)")
	}
	return parts
}

// minValueRoom is the least width worth giving a value; below it the source and warning
// move to their own line under the row instead of squeezing the value out.
const minValueRoom = 12

// settingRow is one line in a group: a label and what it currently holds. placeholder is
// an example to type while the value is unset, and readOnly marks a row Settings shows
// but never edits in place (Workspace, Sources, Blockers), so the two read apart.
type settingRow struct {
	label, value string
	placeholder  string
	readOnly     bool
}

// settingsState is the loaded workspace.json and where the Owner is. It is replaced, never
// mutated in place, so copies of the model stay independent.
type settingsState struct {
	ws    store.WorkspaceSettings
	group int
	row   int
	focus int // settingsList or settingsRows

	editing bool   // a text setting is open for editing
	input   string // what the Owner has typed so far
}

// loadSettings re-reads workspace.json and config.toml. It runs on entering the screen, on r and after
// the editor closes; the group, row and focus are kept.
func (m model) loadSettings() model {
	ws, err := m.store.Settings()
	if err != nil {
		m.err = err
		return m
	}
	if m.configHome != "" {
		if m = m.loadMachine(); m.err != nil {
			return m
		}
	}
	st := settingsState{ws: ws}
	if m.settings != nil {
		st.group, st.row, st.focus = m.settings.group, m.settings.row, m.settings.focus
	}
	m.settings = &st
	return m
}

// rows are the settings of the selected group, each with its current value.
func (m model) settingsRows(group int) []settingRow {
	ws := m.settings.ws
	switch settingsGroups[group] {
	case "Search":
		return []settingRow{
			{label: "Places", value: strings.Join(ws.Places, ", "), placeholder: "Australia, APAC"},
			{label: "Remote", value: remoteLabel(ws.Remote)},
			{label: "Sources · " + strconv.Itoa(ws.Sources), readOnly: true},
			{label: "Blockers · " + strconv.Itoa(ws.Blockers), readOnly: true},
		}
	case "Documents":
		return m.gateRows()
	case "Tracking":
		return m.trackingRows()
	}
	return []settingRow{
		{label: labelWorkspace, value: m.store.Root(), readOnly: true},
		{label: labelEditor, value: ResolveEditor(m.editor), placeholder: "nvim, code --wait"},
		{label: labelDownloads, value: m.downloads, placeholder: "~/Downloads"},
		{label: labelHarness, value: m.harness, placeholder: `claude "/{skill} {dir}"`},
		{label: labelPlugin, value: m.plugin, placeholder: "path to rolecall/plugin"},
		{label: "ASCII glyphs", value: yesNo(m.ascii)},
	}
}

// remoteLabel is search_area.remote as the Owner reads it; missing means the default.
func remoteLabel(remote string) string {
	switch remote {
	case store.RemoteAnywhere, store.RemoteNo:
		return remote
	}
	return "in my places"
}

func (m model) settingsListShown(w int) bool { return w >= settingsSplitWidth }

// settingsHelpKeys is the keys Settings accepts. Application actions are absent.
func (m model) settingsHelpKeys() []helpKey {
	if m.settings != nil && m.settings.editing {
		return []helpKey{help(groupAction, "enter", hintSave), help(groupNav, "esc", "cancel")}
	}
	keys := []helpKey{}
	if m.settings != nil && m.settings.ws.Err == nil {
		keys = append(keys, help(groupAction, "j k", "move"))
	}
	keys = append(keys, m.trackHelpKeys()...)
	keys = append(keys, m.gateHelpKeys()...)
	keys = append(keys, m.searchHelpKeys()...)
	keys = append(keys, m.machineHelpKeys()...)
	file := settingsFile
	if m.settings != nil && settingsGroups[m.settings.group] == groupMachineName && m.configHome != "" {
		file = "config.toml"
	}
	keys = append(keys, help(groupAction, "e", "edit "+file))
	if m.settings != nil && m.settings.ws.Err == nil {
		keys = append(keys, help(groupNav, "tab", "focus"))
	}
	keys = append(keys, help(groupNav, "1-6", "screen"), help(groupNav, "?", "all keys"))
	if m.err != nil {
		keys = append(keys, help(groupNav, "esc", "clear error"))
	} else if m.back != nil {
		keys = append(keys, help(groupNav, "esc", "back"))
	}
	return append(keys, help(groupMore, "r", "rescan"), help(groupMore, "q", "quit"))
}

// settingsKey handles the keys Settings accepts. It never writes: e hands workspace.json
// to the Owner's editor and the screen re-reads when it exits.
func (m model) settingsKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.settings != nil && m.settings.editing {
		return m.settingsEditKey(k)
	}
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
		if m.settings != nil && settingsGroups[m.settings.group] == groupMachineName && m.configHome != "" {
			return m, m.editConfig()
		}
		return m, m.editFile(m.store.Root(), settingsFile)
	case "esc":
		if m.back != nil {
			return m.closeSettings(), nil
		}
	}
	if m.settings == nil || m.settings.ws.Err != nil {
		return m, nil // an invalid file offers e and nothing else
	}
	if next, ok := m.tracksKey(k); ok {
		return next, nil
	}
	if next, ok := m.gatesKey(k); ok {
		return next, nil
	}
	if next, ok := m.searchKey(k); ok {
		return next, nil
	}
	st := *m.settings
	switch k.String() {
	case "enter":
		if st.focus == settingsRows {
			return m.activateSetting()
		}
	case "tab", "shift+tab":
		st.focus = (st.focus + 1) % settingsPanes
	case "down", "j":
		st = m.moveSetting(st, 1)
	case "up", "k":
		st = m.moveSetting(st, -1)
	}
	m.settings = &st
	return m, nil
}

// moveSetting moves the group while the list has focus, the row otherwise.
func (m model) moveSetting(st settingsState, delta int) settingsState {
	if st.focus == settingsList {
		st.group = min(max(st.group+delta, 0), len(settingsGroups)-1)
		st.row = 0
		return st
	}
	m.settings = &st
	st.row = min(max(st.row+delta, 0), len(m.settingsRows(st.group))-1)
	return st
}

// settingsView is the Settings screen in the same frame Profile uses.
func (m model) settingsView(p pal) string {
	w, h := m.viewWidth(), m.profileBodyHeight()
	rule := p.text(p.rule).Render(strings.Repeat("─", w))
	return m.topBar(p) + "\n\n" + m.settingsContextRow(p, w) + "\n" + rule + "\n" +
		m.settingsBody(p, w, h) + "\n" + rule + "\n" + m.statusRow(p, w, m.keyLine(p, w))
}

// settingsContextRow names the focused pane, or the group and its position when the group
// list is hidden; the file path is on the right.
func (m model) settingsContextRow(p pal, w int) string {
	left := "  " + p.heading("SETTINGS")
	switch {
	case m.settings == nil || m.settings.ws.Err != nil:
	case !m.settingsListShown(w):
		left = "  " + p.text(p.fg).Bold(true).Render(settingsGroups[m.settings.group]) +
			p.text(p.dim).Render(" "+strconv.Itoa(m.settings.group+1)+"/"+strconv.Itoa(len(settingsGroups)))
	case m.settings.focus == settingsList:
		left = "  " + p.heading("GROUPS")
	}
	left = xansi.Truncate(left, max(0, w-rightMargin-gutter), "…")
	avail := max(0, w-rightMargin-xansi.StringWidth(left)-gutter)
	return spread(left, p.text(p.dim).Render(xansi.Truncate(settingsFile, avail, "…")), w-rightMargin)
}

// settingsBody is the group list beside the settings, or the invalid-file notice.
func (m model) settingsBody(p pal, w, h int) string {
	if m.settings != nil && m.settings.ws.Err != nil {
		text := "  " + p.text(p.bad).Render("workspace.json is not valid JSON") + "\n  " +
			p.text(p.fg).Render(m.settings.ws.Path) + "\n  " + p.text(p.fg).Render(m.settings.ws.Err.Error()) +
			"\n\n  " + p.text(p.dim).Render("Press e to fix it in your editor. Nothing is written.")
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, text)
	}
	if m.settings == nil {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, "")
	}
	paneW := w
	if m.settingsListShown(w) {
		paneW = w - settingsListWidth - profileDivider
	}
	pane := lipgloss.NewStyle().Width(paneW).Height(h).MaxHeight(h).PaddingLeft(profileGutter).
		Render(m.settingsRowsView(p, paneW-profileGutter))
	if !m.settingsListShown(w) {
		return pane
	}
	var groups []string
	for i, name := range settingsGroups {
		switch {
		case i != m.settings.group:
			groups = append(groups, "  "+p.text(p.dim).Render(name))
		case m.settings.focus == settingsList:
			groups = append(groups, p.text(p.accent).Bold(true).Render(profileMarker+name))
		default:
			// The current group, unmarked: the marker belongs to the focused pane, and the
			// settings rows carry it while they have focus. Bold keeps the group findable.
			groups = append(groups, "  "+p.text(p.fg).Bold(true).Render(name))
		}
	}
	list := lipgloss.NewStyle().Width(settingsListWidth).Height(h).MaxHeight(h).Render(strings.Join(groups, "\n"))
	sep := p.text(p.rule).Render(strings.TrimSuffix(strings.Repeat("│\n", h), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, list, sep, pane)
}

// settingsRowsView lists the selected group's settings, one per line. The row marker shows
// only while the settings pane has focus.
func (m model) settingsRowsView(p pal, width int) string {
	var lines []string
	for i, r := range m.settingsRowsShown(m.settings.group) {
		var extra []string
		mark, label := "  ", p.text(p.dim).Render(r.label)
		if (m.settings.focus == settingsRows || !m.settingsListShown(m.viewWidth())) && i == m.settings.row {
			mark, label = profileMarker, p.text(p.accent).Bold(true).Render(r.label)
		}
		tag := ""
		if r.readOnly {
			tag = "  " + p.text(p.dim).Render("(read-only)")
		}
		pad := strings.Repeat(" ", max(1, settingsLabelWidth-profileMarkerWidth-xansi.StringWidth(r.label)))
		line := mark + label
		switch {
		case r.value != "":
			var parts []string
			if !(m.settings.editing && i == m.settings.row) {
				parts = m.sourceParts(r)
			}
			tail := tag
			if len(parts) > 0 {
				tail += " " + strings.Join(parts, " ")
			}
			if room := width - settingsLabelWidth - xansi.StringWidth(tail); room >= minValueRoom || len(parts) == 0 {
				line += pad + p.text(p.fg).Render(fitValue(r, room)) + p.text(p.dim).Render(tail)
			} else {
				line += pad + p.text(p.fg).Render(fitValue(r, width-settingsLabelWidth)) + tag
				extra = parts
			}
		case r.placeholder != "":
			line += pad + p.text(p.dim).Render("not set ("+r.placeholder+")") + tag
		default:
			line += pad + tag
		}
		lines = append(lines, xansi.Truncate(line, width, "…"))
		for _, part := range extra {
			lines = append(lines, xansi.Truncate(strings.Repeat(" ", settingsLabelWidth)+p.text(p.dim).Render(part), width, "…"))
		}
	}
	if settingsGroups[m.settings.group] == groupMachineName {
		lines = append(lines, "")
		for _, l := range strings.Split(xansi.Hardwrap(editorExamples, max(1, width), true), "\n") {
			lines = append(lines, p.text(p.dim).Render(l))
		}
	}
	return strings.Join(lines, "\n")
}

// fitValue cuts a value that does not fit. A path keeps its end, where the folder names
// that tell Workspaces apart are; other values keep their start.
func fitValue(r settingRow, room int) string {
	if xansi.StringWidth(r.value) <= room || room < 2 {
		return r.value
	}
	switch r.label {
	case labelWorkspace, labelDownloads, labelPlugin:
		return "…" + xansi.TruncateLeft(r.value, xansi.StringWidth(r.value)-room+1, "")
	}
	return xansi.Truncate(r.value, room, "…")
}
