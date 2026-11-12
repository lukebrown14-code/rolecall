package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	xansi "github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
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

func (m model) settingsListShown(w int) bool { return w >= settingsSplitWidth }

// settingsGroupFile is the file the selected group's values live in, for the context row
// and e.
func (m model) settingsGroupFile() string {
	switch {
	case m.settings == nil:
		return settingsFile
	case settingsGroups[m.settings.Group] == groupMachineName:
		return configFile
	case m.settings.Group == settings.GroupTracking:
		return settingsFile + " · " + configFile
	}
	return settingsFile
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
		if m.settings != nil && settingsGroups[m.settings.Group] == groupMachineName {
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
	if m.settings == nil || m.settings.Ws.Err != nil {
		return m, nil // an invalid file offers e and nothing else
	}
	if k.String() == "u" && m.settings.Undo != nil {
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
		if st.Focus == settingsRows || !m.settingsListShown(m.viewWidth()) {
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
	split, focus := 0, focusNone
	if m.settings != nil && m.settings.Ws.Err == nil && m.settingsListShown(w) {
		split, focus = m.settingsListWidth(), focusRight
		if m.settings.Focus == settingsList {
			focus = focusLeft
		}
	}
	return m.frame(p, []string{m.settingsContextRow(p, w)}, m.settingsBody(p, w, h), split, focus, m.statusRow(p, w, m.keyLine(p, w)))
}

// settingsContextRow is one stable heading, or the group and its position when the group
// list is hidden; the selected group's file is on the right.
func (m model) settingsContextRow(p pal, w int) string {
	pad := strings.Repeat(" ", inset)
	left := pad + p.Heading("SETTINGS")
	if m.settings != nil && m.settings.Ws.Err == nil && !m.settingsListShown(w) {
		left = pad + p.Text(p.Fg).Bold(true).Render(settingsGroups[m.settings.Group]) +
			p.Text(p.Dim).Render(" "+strconv.Itoa(m.settings.Group+1)+"/"+strconv.Itoa(len(settingsGroups)))
	}
	return contextRow(left, m.withChip(p, p.Text(p.Dim).Render(m.settingsGroupFile())), w)
}

// settingsBody is the group list beside the settings, or the invalid-file notice.
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
		case i != m.settings.Group:
			groups = append(groups, pad+p.Text(p.Dim).Render(name))
		case m.settings.Focus == settingsList:
			groups = append(groups, strings.Repeat(" ", edge)+p.Text(p.Accent).Bold(true).Render(profileMarker+name))
		default:
			// While the rows have focus only they carry the ▸ cursor; the selected group
			// stays bold and indented, so two cursors never show at once.
			groups = append(groups, strings.Repeat(" ", edge)+p.Text(p.Fg).Bold(true).Render("  "+name))
		}
	}
	list := lipgloss.NewStyle().Width(m.settingsListWidth()).Height(h).MaxHeight(h).Render(strings.Join(groups, "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, list, p.Divider(h), pane)
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
	marked := m.settings.Focus == settingsRows || !m.settingsListShown(m.viewWidth())
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
		if i == m.settings.Row {
			selLine = len(lines)
		}
		mark, label := "  ", p.Text(p.Dim).Render(r.Label)
		switch {
		case selected:
			mark, label = p.Text(p.Accent).Bold(true).Render(profileMarker), p.Text(p.Accent).Bold(true).Render(r.Label)
		case r.Action:
			label = p.Text(p.Sub).Render(r.Label)
		}
		line := mark + label
		if r.Action {
			lines = append(lines, xansi.Truncate(line, width, "…"))
			continue
		}
		pad := strings.Repeat(" ", max(1, labelW-profileMarkerWidth-xansi.StringWidth(r.Label)))
		var value string
		switch {
		case r.Value == "":
			value = p.Text(p.Dim).Italic(true).Render("not set")
		default:
			value = p.Text(p.Fg).Render(fitValue(r.Label, r.ValueText(), valueW))
		}
		value += strings.Repeat(" ", max(0, valueW-xansi.StringWidth(value)))
		tail := p.Text(p.Dim).Render(r.Provenance())
		if r.Warning != "" {
			tail += p.Text(p.Warn).Render(warnSep + r.Warning)
		}
		line += pad + value
		if r.Tail() != "" {
			// The source and any warning are never cut: without room they get their own line.
			if width-labelW-valueW-gutter >= xansi.StringWidth(r.Tail()) {
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
	if settingsGroups[m.settings.Group] == groupMachineName {
		lines = append(lines, "")
		for _, l := range wrapWords(editorExamples, max(1, width-profileMarkerWidth)) {
			lines = append(lines, strings.Repeat(" ", profileMarkerWidth)+p.Text(p.Dim).Render(l))
		}
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

var sourceText = settings.SourceText

var fileSource = settings.FileSource

var remoteLabel = settings.RemoteLabel

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
	labelAddTrack    = settings.LabelAddTrack
)

func (m model) settingsMachine() settings.Machine {
	editor := ResolveEditor(m.editor)
	return settings.Machine{Workspace: m.store.Root(), Editor: editor, EditorSource: m.editorSource, EditorAvailable: editorOnPath(editor), Downloads: m.downloads, Harness: m.harness, Plugin: m.plugin, ASCII: m.ascii, FollowUpDays: m.followUpDays, Sources: m.sources}
}
func (m model) settingsRows(group int) []settingRow {
	return m.settings.Rows(group, m.settingsMachine())
}
func (m model) trackingRows() []settingRow { return m.settingsRows(settings.GroupTracking) }
func (m model) selectableRow(group, row, dir int) int {
	return settings.SelectableRow(m.settingsRows(group), row, dir)
}
