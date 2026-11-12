package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/store"
)

// settingsFile is the Workspace file the Search, Documents and Tracking groups write.
const settingsFile = "workspace.json"

// configFile is the machine file This machine and follow-up days are kept in.
const configFile = "config.toml"

// The two panes tab moves between.
const (
	settingsList = iota
	settingsRows
	settingsPanes
)

// Settings screen sizing mirrors Profile: a group list side panel, then the settings.
const (
	settingsSplitWidth = profileSplitWidth
	// maxLabelWidth caps the label column, which is otherwise as wide as the group's longest label.
	maxLabelWidth = 24
)

// settingsListWidth is the group list's column, divider excluded.
func (m model) settingsListWidth() int { return m.layout().panel(groupsShare, groupsMin, groupsMax) }

// settingsGroups are the groups in display order.
var settingsGroups = []string{"Search", "Documents", "Tracking", groupMachineName}

const groupMachineName = "This machine"

// Labels of the settings rows the code refers to.
const (
	labelEditor    = "Editor"
	labelFollowUp  = "Follow-up due after"
	labelHarness   = "Harness"
	labelWorkspace = "Workspace"
	labelDownloads = "Downloads"
	labelPlugin    = "Plugin"
	labelASCII     = "ASCII glyphs"
	labelAddTrack  = "+ add a Track"
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

// settingRow is one line in a group. Every editable value says where it comes from
// (source), an unset one shows an example instead, and a read-only row says why it is
// fixed, so "not set", "default" and "read-only" never read alike.
type settingRow struct {
	label, value string
	unit         string // after the value: "days", "words"
	source       string // the file or variable the value comes from, or "default"
	example      string // shown while the value is unset
	readOnly     bool   // shown, never edited here; the cursor passes over it
	reason       string // why a read-only row is fixed
	warning      string // a problem with the value, e.g. an Editor not on PATH
	section      string // a heading drawn above the row
	action       bool   // "+ add a Track": no value, enter acts
}

// settingsUndo is the last Settings save, so u can put the old value back once.
type settingsUndo struct {
	what    string // "Editor back to vi"
	restore func(model) model
}

// settingsState is the loaded workspace.json and where the Owner is. It is replaced, never
// mutated in place, so copies of the model stay independent.
type settingsState struct {
	ws       store.WorkspaceSettings
	trackUse map[string]int // Applications on each Track, archived ones included
	group    int
	row      int
	focus    int // settingsList or settingsRows
	undo     *settingsUndo
}

// loadSettings re-reads workspace.json and config.toml. It runs on entering the screen, on r and after
// the editor closes; the group, row, focus and undo are kept.
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
	st := settingsState{ws: ws, trackUse: map[string]int{}}
	for _, t := range ws.Tracks {
		if n, err := m.store.TrackUse(t); err == nil {
			st.trackUse[t] = n
		}
	}
	if m.settings != nil {
		st.group, st.row, st.focus, st.undo = m.settings.group, m.settings.row, m.settings.focus, m.settings.undo
	}
	m.settings = &st
	st.row = m.selectableRow(st.group, st.row, 1)
	return m
}

// sourceText is a source as the value's provenance reads: "from $EDITOR", "config.toml".
func sourceText(src string) string {
	switch src {
	case config.SourceVisual, config.SourceEditor, config.SourcePlugin, config.SourceFlag:
		return "from " + src
	}
	return src
}

// fileSource is a workspace.json value's source: the file, or the default it falls back to.
func fileSource(set bool) string {
	if set {
		return settingsFile
	}
	return config.SourceDefault
}

// settingsRows are the settings of a group, each with its current value and source.
func (m model) settingsRows(group int) []settingRow {
	ws := m.settings.ws
	switch settingsGroups[group] {
	case "Search":
		return []settingRow{
			{label: fieldPlaces, value: strings.Join(ws.Places, ", "), source: settingsFile, example: "Australia, APAC"},
			{label: fieldRemote, value: remoteLabel(ws.Remote), source: fileSource(ws.Remote != "")},
			{label: "Sources", value: strconv.Itoa(ws.Sources), readOnly: true, reason: "read-only"},
			{label: "Blockers", value: strconv.Itoa(ws.Blockers), readOnly: true, reason: "read-only"},
		}
	case "Documents":
		return m.gateRows()
	case "Tracking":
		return m.trackingRows()
	}
	editor := ResolveEditor(m.editor)
	editorRow := settingRow{label: labelEditor, value: editor, source: sourceText(m.editorSource), example: "nvim, code --wait"}
	if !editorOnPath(editor) {
		editorRow.warning = "not found on PATH"
	}
	ascii := config.SourceDefault
	if m.ascii {
		ascii = configFile
	}
	return []settingRow{
		{label: labelWorkspace, value: m.store.Root(), readOnly: true, reason: "fixed this session", source: sourceText(m.sources.Workspace)},
		editorRow,
		{label: labelDownloads, value: m.downloads, source: sourceText(m.sources.Downloads), example: "~/Downloads"},
		{label: labelHarness, value: m.harness, source: sourceText(m.sources.Harness), example: `claude "/{skill} {dir}"`},
		{label: labelPlugin, value: m.plugin, source: sourceText(m.sources.Plugin), example: "path to rolecall/plugin"},
		{label: labelASCII, value: yesNo(m.ascii), source: ascii},
	}
}

// Remote rule labels: how search_area.remote reads, here and in the Search area form.
var remoteLabels = map[string]string{
	store.RemoteInPlaces: "count if in my Places",
	store.RemoteAnywhere: "count from anywhere",
	store.RemoteNo:       "never count",
}

// remoteLabel is search_area.remote as the Owner reads it; missing means the default.
func remoteLabel(remote string) string {
	if l, ok := remoteLabels[remote]; ok {
		return l
	}
	return remoteLabels[store.RemoteInPlaces]
}

// selectable reports whether the cursor can rest on a row: everything but read-only rows.
func (r settingRow) selectable() bool { return !r.readOnly }

// selectableRow is row if the cursor can rest there, else the nearest row in direction
// dir (then the other way) that it can.
func (m model) selectableRow(group, row, dir int) int {
	rows := m.settingsRows(group)
	if len(rows) == 0 {
		return 0
	}
	row = min(max(row, 0), len(rows)-1)
	for _, d := range []int{dir, -dir} {
		for i := row; i >= 0 && i < len(rows); i += d {
			if rows[i].selectable() {
				return i
			}
		}
	}
	return row
}

func (m model) settingsListShown(w int) bool { return w >= settingsSplitWidth }

// settingsGroupFile is the file the selected group's values live in, for the context row
// and e.
func (m model) settingsGroupFile() string {
	switch {
	case m.settings == nil:
		return settingsFile
	case settingsGroups[m.settings.group] == groupMachineName:
		return configFile
	case m.settings.group == groupTracking:
		return settingsFile + " · " + configFile
	}
	return settingsFile
}

// settingsHelpKeys is the keys Settings accepts. Application actions are absent, and so
// are the global keys.
func (m model) settingsHelpKeys() []helpKey {
	keys := []helpKey{}
	keys = append(keys, m.trackHelpKeys()...)
	keys = append(keys, m.gateHelpKeys()...)
	keys = append(keys, m.searchHelpKeys()...)
	keys = append(keys, m.machineHelpKeys()...)
	if m.settings != nil && m.settings.undo != nil {
		keys = append(keys, help(groupAction, "u", "undo").at(prioMinor))
	}
	if m.settings != nil && settingsGroups[m.settings.group] == groupMachineName {
		if m.configHome != "" {
			keys = append(keys, help(groupAction, "e", "edit "+configFile))
		}
	} else {
		keys = append(keys, help(groupAction, "e", "edit "+settingsFile))
	}
	if m.settings != nil && m.settings.ws.Err == nil {
		keys = append(keys, help(groupNav, "j k", "move"), help(groupNav, "tab", "focus").at(prioScreen))
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
		if m.settings != nil && settingsGroups[m.settings.group] == groupMachineName {
			if m.configHome == "" {
				return m, nil
			}
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
	if k.String() == "u" && m.settings.undo != nil {
		return m.undoSetting(), nil
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
		if st.focus == settingsRows || !m.settingsListShown(m.viewWidth()) {
			return m.activateSetting()
		}
	case "tab", "shift+tab":
		st.focus = (st.focus + 1) % settingsPanes
		st.row = m.selectableRow(st.group, st.row, 1)
	case "down", "j":
		st = m.moveSetting(st, 1)
	case "up", "k":
		st = m.moveSetting(st, -1)
	}
	m.settings = &st
	return m, nil
}

// moveSetting moves the group while the list has focus, the row otherwise. The row skips
// read-only rows, so enter always has something to do.
func (m model) moveSetting(st settingsState, delta int) settingsState {
	if st.focus == settingsList {
		st.group = min(max(st.group+delta, 0), len(settingsGroups)-1)
		st.row = m.selectableRow(st.group, 0, 1)
		return st
	}
	rows := m.settingsRows(st.group)
	for i := st.row + delta; i >= 0 && i < len(rows); i += delta {
		if rows[i].selectable() {
			st.row = i
			break
		}
	}
	return st
}

// saved reports a Settings save on the feedback line, naming the file it went to, and
// keeps restore so u can put the old value back.
func (m model) saved(msg, file string, undo *settingsUndo) model {
	m.toast = msg + " · " + file
	if m.settings != nil {
		st := *m.settings
		st.undo = undo
		m.settings = &st
	}
	if undo != nil {
		m.toastHint = "u undo"
	}
	return m
}

// undoSetting puts back the value the last Settings save replaced.
func (m model) undoSetting() model {
	u := m.settings.undo
	m = u.restore(m)
	st := *m.settings
	st.undo = nil
	m.settings = &st
	if m.err == nil {
		m.toast, m.toastHint = "Undone · "+u.what, ""
	}
	return m
}

// settingsView is the Settings screen in the same frame Profile uses.
func (m model) settingsView(p pal) string {
	w, h := m.viewWidth(), m.profileBodyHeight()
	split, focus := 0, focusNone
	if m.settings != nil && m.settings.ws.Err == nil && m.settingsListShown(w) {
		split, focus = m.settingsListWidth(), focusRight
		if m.settings.focus == settingsList {
			focus = focusLeft
		}
	}
	return m.frame(p, []string{m.settingsContextRow(p, w)}, m.settingsBody(p, w, h), split, focus, m.statusRow(p, w, m.keyLine(p, w)))
}

// settingsContextRow is one stable heading, or the group and its position when the group
// list is hidden; the selected group's file is on the right.
func (m model) settingsContextRow(p pal, w int) string {
	pad := strings.Repeat(" ", inset)
	left := pad + p.heading("SETTINGS")
	if m.settings != nil && m.settings.ws.Err == nil && !m.settingsListShown(w) {
		left = pad + p.text(p.fg).Bold(true).Render(settingsGroups[m.settings.group]) +
			p.text(p.dim).Render(" "+strconv.Itoa(m.settings.group+1)+"/"+strconv.Itoa(len(settingsGroups)))
	}
	return contextRow(left, m.withChip(p, p.text(p.dim).Render(m.settingsGroupFile())), w)
}

// settingsBody is the group list beside the settings, or the invalid-file notice.
func (m model) settingsBody(p pal, w, h int) string {
	pad := strings.Repeat(" ", inset)
	if m.settings != nil && m.settings.ws.Err != nil {
		text := pad + p.text(p.bad).Render("workspace.json is not valid JSON") + "\n" + pad +
			p.text(p.fg).Render(m.settings.ws.Path) + "\n" + pad + p.text(p.fg).Render(m.settings.ws.Err.Error()) +
			"\n\n" + pad + p.text(p.dim).Render("Press e to fix it in your editor. Nothing is written.")
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, text)
	}
	if m.settings == nil {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, "")
	}
	// Without the list the rows' markers sit at the chrome column, like the list's.
	paneW, gutterW := w, edge
	if m.settingsListShown(w) {
		paneW, gutterW = w-m.settingsListWidth()-profileDivider, profileGutter
	}
	pane := lipgloss.NewStyle().Width(paneW).Height(h).MaxHeight(h).PaddingLeft(gutterW).
		Render(m.settingsRowsView(p, paneW-gutterW-rightMargin, h))
	if !m.settingsListShown(w) {
		return pane
	}
	var groups []string
	for i, name := range settingsGroups {
		switch {
		case i != m.settings.group:
			groups = append(groups, pad+p.text(p.dim).Render(name))
		case m.settings.focus == settingsList:
			groups = append(groups, strings.Repeat(" ", edge)+p.text(p.accent).Bold(true).Render(profileMarker+name))
		default:
			// The selected group keeps its ▸ while the rows have focus, in a quieter
			// colour, so it is clear which group the rows belong to.
			groups = append(groups, strings.Repeat(" ", edge)+p.text(p.dim).Render(profileMarker)+p.text(p.fg).Bold(true).Render(name))
		}
	}
	list := lipgloss.NewStyle().Width(m.settingsListWidth()).Height(h).MaxHeight(h).Render(strings.Join(groups, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, list, p.divider(h), pane)
}

// settingValueText is a row's value with its unit, as the value column shows it.
func (r settingRow) valueText() string {
	if r.value != "" && r.unit != "" {
		return r.value + " " + r.unit
	}
	return r.value
}

// provenance is what follows a row's value: why a read-only row is fixed, where a value
// comes from, or an example while it is unset.
func (r settingRow) provenance() string {
	switch {
	case r.readOnly && r.source != "":
		return r.reason + " · " + r.source
	case r.readOnly:
		return r.reason
	case r.value == "" && r.example != "":
		return "example: " + r.example
	}
	return r.source
}

// warnSep leads a row's warning after its source.
const warnSep = "  ⚠ "

// tail is the source and warning after a row's value, as plain text, for sizing.
func (r settingRow) tail() string {
	t := r.provenance()
	if r.warning != "" {
		t += warnSep + r.warning
	}
	return t
}

// settingsRowsView lists the selected group's settings, one per line, in three columns:
// label, value, and where the value comes from. The row marker shows only while the
// settings pane has focus.
func (m model) settingsRowsView(p pal, width, h int) string {
	rows := m.settingsRows(m.settings.group)
	valueW, labelW, tailW := 0, 0, 0
	for _, r := range rows {
		tailW = max(tailW, xansi.StringWidth(r.tail()))
		if !r.action {
			labelW = max(labelW, profileMarkerWidth+xansi.StringWidth(r.label)+gutter)
		}
		w := xansi.StringWidth(r.valueText())
		if r.value == "" && !r.action {
			w = xansi.StringWidth("not set")
		}
		valueW = max(valueW, w)
	}
	labelW = min(labelW, maxLabelWidth)
	room := width - labelW
	valueW = min(valueW, max(minValueRoom, room-tailW-gutter))
	marked := m.settings.focus == settingsRows || !m.settingsListShown(m.viewWidth())
	var lines []string
	selLine := 0
	for i, r := range rows {
		if r.section != "" {
			if len(lines) > 0 {
				lines = append(lines, "")
			}
			lines = append(lines, strings.Repeat(" ", profileMarkerWidth)+p.heading(r.section))
		}
		selected := marked && i == m.settings.row
		if i == m.settings.row {
			selLine = len(lines)
		}
		mark, label := "  ", p.text(p.dim).Render(r.label)
		switch {
		case selected:
			mark, label = p.text(p.accent).Bold(true).Render(profileMarker), p.text(p.accent).Bold(true).Render(r.label)
		case r.action:
			label = p.text(p.sub).Render(r.label)
		}
		line := mark + label
		if r.action {
			lines = append(lines, xansi.Truncate(line, width, "…"))
			continue
		}
		pad := strings.Repeat(" ", max(1, labelW-profileMarkerWidth-xansi.StringWidth(r.label)))
		var value string
		switch {
		case r.value == "":
			value = p.text(p.dim).Italic(true).Render("not set")
		default:
			value = p.text(p.fg).Render(fitValue(r.label, r.valueText(), valueW))
		}
		value += strings.Repeat(" ", max(0, valueW-xansi.StringWidth(value)))
		tail := p.text(p.dim).Render(r.provenance())
		if r.warning != "" {
			tail += p.text(p.warn).Render(warnSep + r.warning)
		}
		line += pad + value
		if r.tail() != "" {
			// The source and any warning are never cut: without room they get their own line.
			if width-labelW-valueW-gutter >= xansi.StringWidth(r.tail()) {
				line += strings.Repeat(" ", gutter) + tail
				lines = append(lines, xansi.Truncate(line, width, "…"))
			} else {
				lines = append(lines, xansi.Truncate(line, width, "…"),
					xansi.Truncate(strings.Repeat(" ", labelW)+tail, width, "…"))
			}
			continue
		}
		lines = append(lines, xansi.Truncate(line, width, "…"))
	}
	if settingsGroups[m.settings.group] == groupMachineName {
		lines = append(lines, "")
		for _, l := range wrapWords(editorExamples, max(1, width-profileMarkerWidth)) {
			lines = append(lines, strings.Repeat(" ", profileMarkerWidth)+p.text(p.dim).Render(l))
		}
	}
	return strings.Join(p.clipLines(lines, selLine-h/bothSides, h, strings.Repeat(" ", profileMarkerWidth)), "\n")
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
