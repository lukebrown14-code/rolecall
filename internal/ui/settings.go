package ui

import (
	"errors"
	"image/color"
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

// settingsBack is where Esc returns to from Settings.
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

// closeSettings is Esc on Settings: back to the screen it was opened from.
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
	// A Track list that breaks the name rules (one listed twice) stops the screen like a
	// broken file: its rows would offer edits on a list rolecall cannot use.
	if _, err := m.store.Tracks(); st.Ws.Err == nil && badTrackList(err) {
		st.Ws.Err = err
	}
	if m.configHome != "" {
		// A config.toml problem takes the footer; otherwise the error already there stays,
		// unless it was config.toml's own, now re-read.
		was, stale := m.err, m.configFeedback()
		if was != nil && stale != nil && was.Error() == stale.Error() {
			was = nil
		}
		m.err = nil
		if m = m.loadMachine(); m.err == nil {
			m.err = was
		}
	}

	m.settings = &st
	m.wsInvalid = st.Ws.Err != nil
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
	{group: settings.GroupRules, help: model.rulesHelpKeys, key: model.rulesKey},
	{group: settings.GroupSources, help: model.sourcesHelpKeys, key: model.sourcesKey},
	{group: settings.GroupDocuments, help: model.gateHelpKeys, key: model.gatesKey},
	{group: settings.GroupTracking, help: model.trackHelpKeys, key: model.tracksKey},
	{group: settings.GroupMachine, help: model.machineHelpKeys},
}

// settingsHelpKeys is the keys Settings accepts. Application actions are absent, and so
// are the global keys.
func (m model) settingsHelpKeys() []helpKey {
	if m.settings != nil && m.settings.Edit != nil {
		if m.settings.Edit.Discarding {
			return []helpKey{help(groupAction, "y", "discard").at(prioCore), help(groupNav, keyNoEsc, "keep editing").at(prioPinned)}
		}
		return []helpKey{help(groupAction, "enter", "save").at(prioCore), help(groupAction, keyClearField, "clear").at(prioCore),
			help(groupNav, "esc", "cancel").at(prioPinned)}
	}
	keys := []helpKey{}
	for _, h := range settingsHandlers {
		keys = append(keys, h.help(m)...)
	}
	if m.settings != nil && m.settings.Undo != nil {
		keys = append(keys, help(groupAction, "u", "undo").at(prioPinned)) // the toast's offer stays reachable
	}
	if m.eOpensConfig() {
		if m.configHome != "" {
			keys = append(keys, help(groupAction, "e", "edit "+configFile))
		}
	} else if m.settings != nil && m.settings.Ws.Err != nil {
		// The invalid-file screen offers e and nothing else, so its key line says so.
		keys = append(keys, help(groupAction, "e", "fix").at(prioPinned))
	} else {
		keys = append(keys, help(groupAction, "e", "edit "+settingsFile))
	}
	if m.settings != nil && m.settings.Ws.Err == nil {
		// ←→ switch sections as they switch views elsewhere; tab and [ ] do too, unlisted, as
		// help says tab switches panes and would wrap [ ] in brackets.
		// Below the medium width there is no group list beside the rows, so the section key
		// is pinned: it is the only sign the other sections are there.
		section := help(groupNav, "←→", "section").at(prioCore)
		if m.layout().w < mediumWidth {
			section = section.at(prioPinned)
		}
		keys = append(keys, section, help(groupNav, "j k", "select"),
			help(groupNav, "g G", "first, last").at(prioHelpOnly))
	}
	if m.err != nil {
		keys = append(keys, help(groupNav, "esc", "clear error").at(prioPinned))
	} else if m.back != nil {
		keys = append(keys, help(groupNav, "esc", "back").at(prioPinned))
	}
	return keys
}

// settingsKey handles the keys Settings accepts. e hands the group's file to the Owner's
// editor and the screen re-reads when it exits; Enter saves each change at once.
func (m model) settingsKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
		if m.eOpensConfig() {
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
	if k.String() == "u" {
		if m.settings.Undo == nil {
			// u reverts Settings changes here, not moves; with none it still answers.
			m.toast, m.toastKind = "Nothing to undo in Settings", toastNote
			return m, nil
		}
		return m.undoSetting(), nil
	}
	if next, cmd, ok := m.sourceCheckKey(k); ok {
		return next, cmd
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

// eOpensConfig reports whether e opens config.toml rather than workspace.json: on This
// machine (Editor and the rest), or anywhere while config.toml does not parse and
// workspace.json is fine, so e always leads to the broken file.
func (m model) eOpensConfig() bool {
	if m.settings == nil {
		return m.configBroken()
	}
	return settingsGroups[m.settings.Group] == groupMachineName || (m.configBroken() && m.settings.Ws.Err == nil)
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

// rowShown is the value a group's row named label shows, unit and all, or "not set": what
// an undo names, so it reads as the row did before the save.
func (m model) rowShown(group int, label string) string {
	for _, r := range m.settingsRows(group) {
		if r.Label == label && r.Value != "" {
			return r.ValueText()
		}
	}
	return "not set"
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

// settingsView is the Settings screen in the same frame Profile uses. At wide sizes the
// focused row's about text has a pane of its own beside the rows.
func (m model) settingsView(p pal) string {
	w, h := m.viewWidth(), m.profileBodyHeight()
	split := 0
	if m.settingsAboutShown() {
		split = m.settingsRowsWidth(w)
	}
	return m.frame(p, []string{m.settingsContextRow(p, w)}, m.settingsBody(p, w, h), split, focusLeft, m.statusRow(p, m.keyLine(p, w)))
}

// settingsAboutShown reports whether the about pane sits beside the rows: at wide sizes,
// with a valid workspace.json.
func (m model) settingsAboutShown() bool {
	return m.layout().wide() && m.settings != nil && m.settings.Ws.Err == nil
}

// settingsRowsWidth is the rows pane beside the about pane and its divider. The about
// pane is a share of the width, so the rows keep room for their values and sources.
func (m model) settingsRowsWidth(w int) int {
	return w - min(max(w*aboutShare/percent, aboutMin), aboutMax) - profileDivider
}

// The about pane's share of the width, and its least and most cells.
const aboutShare, aboutMin, aboutMax = 30, 30, 48

// settingsTabs is the sections as `[Tracking]  Tracks …`: the active one in the single
// selection style, brackets and all, so it reads without colour, as the active tab does.
// Every section is named while they fit in room; otherwise only the active one and its place.
func (m model) settingsTabs(p pal, room int) string {
	active := func(name string) string { return p.SelText(true).Render("[" + name + "]") }
	var parts []string
	for i, name := range settingsGroups {
		if i == m.settings.Group {
			parts = append(parts, active(name))
		} else {
			parts = append(parts, p.Text(p.Dim).Render(name))
		}
	}
	full := strings.Join(parts, "  ")
	if xansi.StringWidth(full) <= room {
		return full
	}
	return active(settingsGroups[m.settings.Group]) + p.Text(p.Dim).Render(" "+strconv.Itoa(m.settings.Group+1)+" of "+strconv.Itoa(len(settingsGroups)))
}

// settingsContextRow is the heading and the section tabs. The cursor rests here too (k
// from the first field), where only the section keys act.
func (m model) settingsContextRow(p pal, w int) string {
	lead := strings.Repeat(" ", inset)
	if m.settings == nil || m.settings.Ws.Err != nil {
		return strings.Repeat(" ", inset) + p.Heading("SETTINGS")
	}
	if m.settings.Header {
		lead = strings.Repeat(" ", edge) + p.SelGutter(true)
	}
	head := lead + p.Heading("SETTINGS") + "  "
	right := m.withChip(p, "")
	room := w - rightMargin - xansi.StringWidth(head)
	if right != "" {
		room -= xansi.StringWidth(right) + gutter
	}
	return contextRow(head+m.settingsTabs(p, room), right, w)
}

// settingsBody is the settings of the selected section, or the invalid-file notice.
func (m model) settingsBody(p pal, w, h int) string {
	pad := strings.Repeat(" ", inset)
	if m.settings != nil && m.settings.Ws.Err != nil {
		head, why := "workspace.json is not valid JSON", m.settings.Ws.Err
		var tle *store.TrackListError
		switch {
		case m.settings.Ws.NewerSchema:
			head = "workspace.json is schema " + strconv.Itoa(m.settings.Ws.Schema) + ", from a newer rolecall"
		case errors.As(why, &tle):
			head, why = "workspace.json lists a Track rolecall can't use", tle.Err
		}
		// The path and the reason wrap rather than cut: a long path is what the Owner has to
		// open, and a cut one cannot be copied.
		lines := []string{p.Text(p.Bad).Render(head)}
		for _, part := range []string{m.settings.Ws.Path, why.Error()} {
			for _, l := range wrapWords(part, max(1, w-inset-rightMargin)) {
				lines = append(lines, p.Text(p.Fg).Render(l))
			}
		}
		lines = append(lines, "", p.Text(p.Dim).Render("Press e to fix it in your editor. Nothing is written."))
		text := pad + strings.Join(lines, "\n"+pad)
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, text)
	}
	if m.settings == nil {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, "")
	}
	if !m.settingsAboutShown() {
		return lipgloss.NewStyle().Width(w).Height(h).MaxHeight(h).PaddingLeft(edge).
			Render(m.settingsRowsView(p, w-edge-rightMargin, h, true))
	}
	rw := m.settingsRowsWidth(w)
	rows := lipgloss.NewStyle().Width(rw).Height(h).MaxHeight(h).PaddingLeft(edge).
		Render(m.settingsRowsView(p, rw-edge-gutter, h, false))
	aboutW := w - rw - profileDivider
	about := lipgloss.NewStyle().Width(aboutW).Height(h).MaxHeight(h).PaddingLeft(profileGutter).
		Render(strings.Join(m.settingsAbout(p, aboutW-profileGutter-rightMargin), "\n"))
	return lipgloss.JoinHorizontal(lipgloss.Top, rows, p.Divider(h), about)
}

// settingsFocusedRow is the row under the cursor; ok is false on the header.
func (m model) settingsFocusedRow() (settingRow, bool) {
	rows := m.settingsRows(m.settings.Group)
	if m.settings.Header || m.settings.Row < 0 || m.settings.Row >= len(rows) {
		return settingRow{}, false
	}
	return rows[m.settings.Row], true
}

// settingsAbout is the about pane: what the focused setting is, an example of its value,
// the file it is kept in and any warning, or what the group is while the header has focus.
func (m model) settingsAbout(p pal, w int) []string {
	w = max(1, w)
	para := func(s string, c func(string) string) []string {
		var out []string
		for _, l := range wrapWords(s, w) {
			out = append(out, c(l))
		}
		return out
	}
	fg := func(s string) string { return p.Text(p.Fg).Render(s) }
	dim := func(s string) string { return p.Text(p.Dim).Render(s) }
	r, ok := m.settingsFocusedRow()
	if !ok {
		lines := []string{p.Heading(settingsGroups[m.settings.Group]), ""}
		return append(lines, para(settings.GroupAbout[m.settings.Group], fg)...)
	}
	label := strings.TrimPrefix(r.Label, "+ ")
	lines := []string{p.Heading(label), ""}
	lines = append(lines, para(r.About, fg)...)
	if r.Example != "" {
		lines = append(lines, "")
		lines = append(lines, para("Example: "+r.Example, dim)...)
	}
	if !r.Action {
		lines = append(lines, "")
		lines = append(lines, para("Kept in "+m.settingsFileOf(m.settings.Group, m.settings.Row)+".", dim)...)
	}
	if r.Warning != "" {
		lines = append(lines, "")
		lines = append(lines, para("⚠ "+r.Warning, func(s string) string { return p.Text(p.Warn).Render(s) })...)
	}
	return lines
}

// settingsFileOf is the file a row's value is kept in.
func (m model) settingsFileOf(group, row int) string {
	if _, ok := m.rowSetting(group, row, len(m.settingsRows(group))); ok || group == settings.GroupMachine {
		return configFile
	}
	return settingsFile
}

// settingsRowsView lists the selected group's settings, one per line, in three columns:
// label, value, and where the value comes from. The row marker shows only while the
// settings pane has focus. With aboutBelow, the focused row's about text follows the rows,
// as the pane beside them is not drawn.
func (m model) settingsRowsView(p pal, width, h int, aboutBelow bool) string {
	rows := m.settingsRows(m.settings.Group)
	valueW, labelW, tailW := 0, 0, 0
	for _, r := range rows {
		if !r.ReadOnly {
			// A read-only row's reason never narrows the values: without room it wraps below.
			tailW = max(tailW, xansi.StringWidth(r.Tail()))
		}
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
	if settingsGroups[m.settings.Group] == groupMachineName {
		lines = m.configNotice(p, width)
	}
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
		if i == m.settings.Row {
			selLine = len(lines)
		}
		// Names in the body text colour and values a step quieter, so a row reads by its name.
		mark, label := "  ", p.Text(p.Fg).Render(r.Label)
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
		// Muted text on the selection fill is below 4.5:1, so a selected row's quiet parts
		// take the secondary colour, as a selected list row's do.
		quiet := p.Dim
		if selected {
			quiet = p.Sub
		}
		pad := bg.Render(strings.Repeat(" ", max(1, labelW-profileMarkerWidth-xansi.StringWidth(r.Label))))
		var value string
		if r.Value == "" {
			value = bg.Foreground(quiet).Italic(true).Render("not set")
		} else {
			value = bg.Foreground(p.Sub).Render(fitValue(r.Label, r.ValueText(), valueW))
		}
		colW := valueW
		value += bg.Render(strings.Repeat(" ", max(0, colW-xansi.StringWidth(value))))
		tail := bg.Foreground(quiet).Render(r.Note())
		if editing != nil && isSetting && editing.Key == set.key {
			value, tail = m.inlineField(p, bg, quiet, *editing, r, valueW)
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
		if plain := xansi.Strip(tail); plain != "" {
			// The source and any warning are never cut: without room they get their own line.
			if width-labelW-colW-gutter >= xansi.StringWidth(plain) {
				line += bg.Render(strings.Repeat(" ", gutter)) + tail
				lines = append(lines, fillLine(line))
			} else {
				lines = append(lines, fillLine(line))
				// On its own line the tail wraps rather than lose the end of an error.
				for _, l := range strings.Split(xansi.Wrap(tail, max(1, width-labelW), " "), "\n") {
					lines = append(lines, strings.Repeat(" ", labelW)+l)
				}
			}
			continue
		}
		lines = append(lines, fillLine(line))
	}
	if aboutBelow {
		if r, ok := m.settingsFocusedRow(); ok && r.About != "" {
			about := wrapWords(r.About, max(1, width-profileMarkerWidth))
			if len(lines)+1+len(about) <= h {
				// Short terminals lose the about text before they lose a field.
				lines = append(lines, "")
				for _, l := range about {
					lines = append(lines, strings.Repeat(" ", profileMarkerWidth)+p.Text(p.Dim).Render(l))
				}
			}
		}
	}
	return strings.Join(p.ClipLines(lines, selLine-h/bothSides, h, strings.Repeat(" ", profileMarkerWidth)), "\n")
}

// inlineField is the value cell and the tail of a row being edited: the field where the
// value sits, narrow, with the end of what is typed. An empty field shows the default in
// effect as a placeholder; the opening text shows selected, as typing replaces it. The
// example stays beside the field, and a quit's question takes the tail.
func (m model) inlineField(p pal, bg lipgloss.Style, quiet color.Color, e settings.Edit, r settingRow, valueW int) (value, tail string) {
	fw := min(maxFieldWidth, max(minFieldWidth, valueW))
	cursor := bg.Foreground(p.Fg).Render("▏")
	switch {
	case e.Buf == "" && e.Placeholder != "":
		shown := xansi.Truncate(e.Placeholder, fw-1, "…")
		value = cursor + bg.Foreground(quiet).Italic(true).Render(shown)
		value += bg.Render(strings.Repeat(" ", max(0, fw-xansi.StringWidth(shown)-1)))
	default:
		shown := xansi.TruncateLeft(e.Buf, max(0, xansi.StringWidth(e.Buf)-(fw-1)), "…")
		style := bg.Foreground(p.Fg).Underline(true)
		if e.Selected {
			style = bg.Foreground(p.Fg).Reverse(true)
		}
		value = style.Render(shown) + cursor
		value += bg.Render(strings.Repeat(" ", max(0, fw-xansi.StringWidth(shown)-1)))
	}
	var parts []string
	if r.Unit != "" {
		parts = append(parts, r.Unit)
	}
	if r.Example != "" && (e.Buf != "" || e.Placeholder != r.Example) {
		parts = append(parts, "example: "+r.Example) // not twice while the placeholder shows it
	}
	tail = bg.Foreground(quiet).Render(strings.Join(parts, " · "))
	switch {
	case e.Discarding:
		tail = bg.Foreground(p.Warn).Bold(true).Render("⚠ Discard changes? y discard · n keep editing")
	case e.Err != "":
		tail = bg.Foreground(p.Bad).Render(errorMark + " " + e.Err)
	}
	return value, tail
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

// settingsMachine is the machine settings as the screen shows them, with a warning for a
// Harness or Plugin that will not run.
func (m model) settingsMachine() settings.Machine {
	mc := settings.Machine{Workspace: m.store.Root(), EditorSource: m.editorSource, Downloads: m.downloads, Harness: m.harness,
		Plugin: m.plugin, ASCII: m.ascii, Inherit: m.inherit, FollowUpDays: m.followUpDays, Sources: m.sources,
		HarnessWarning: harnessWarning(m.harness), PluginWarning: pluginWarning(m.plugin),
		FollowUpWarning: m.configWarning(keyFollowUpDays)}
	mc.Editor = ResolveEditor(m.editor)
	mc.EditorAvailable = editorOnPath(mc.Editor)
	return mc
}
func (m model) settingsRows(group int) []settingRow {
	st := *m.settings
	st.Breakdown = m.trackBreakdown(st)
	return st.Rows(group, m.settingsMachine())
}

// trackBreakdown explains each Track's count so the numbers add up: how many of its
// Applications are active, how many closed (Rejected or Withdrawn, as the Pipeline screen's
// Closed view holds) and how many dismissed.
func (m model) trackBreakdown(st settingsState) map[string]string {
	closed, dismissed := map[string]int{}, map[string]int{}
	for _, a := range m.apps {
		switch a.Status {
		case store.StatusRejected, store.StatusWithdrawn:
			closed[a.Track]++
		case store.StatusDismissed:
			dismissed[a.Track]++
		}
	}
	out := map[string]string{}
	for _, t := range st.Ws.Tracks {
		c := min(closed[t], st.TrackUse[t])
		d := min(dismissed[t], st.TrackUse[t]-c)
		out[t] = " · " + strconv.Itoa(st.TrackUse[t]-c-d) + " active"
		if c > 0 {
			out[t] += " · " + strconv.Itoa(c) + " closed"
		}
		if d > 0 {
			out[t] += " · " + strconv.Itoa(d) + " dismissed"
		}
	}
	return out
}
func (m model) trackingRows() []settingRow { return m.settingsRows(settings.GroupTracking) }
func (m model) selectableRow(group, row, dir int) int {
	return settings.SelectableRow(m.settingsRows(group), row, dir)
}
