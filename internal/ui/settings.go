package ui

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// The Settings screen is the sub-model in internal/ui/settings. This adapter opens and
// closes it, hands it the running application (Env) and the shared look (Look), and
// applies the intents it returns: writes through store and config, the reload that
// follows, the feedback line, and the pop-ups over the frame.

// settingsFile is the Workspace file the Search, Documents and Tracking groups write.
const settingsFile = "workspace.json"

// configFile is the machine file This machine and follow-up days are kept in.
const configFile = "config.toml"

type settingsState = settings.Model

var settingsGroups = settings.Groups

const (
	groupMachineName = settings.GroupMachineName
	labelEditor      = settings.LabelEditor
	labelHarness     = settings.LabelHarness
	labelPlugin      = settings.LabelPlugin
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

// loadSettings re-reads workspace.json and config.toml. It runs on entering the screen, on r and after
// the editor closes; the group, row, focus and undo are kept.
func (m model) loadSettings() model {
	st := settingsState{}
	if m.settings != nil {
		st = *m.settings
	}
	loaded, err := st.Load(m.store)
	if err != nil {
		m.err = err
		return m
	}
	st.State = loaded
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
	st = st.Clamp(m.settingsEnv())
	m.settings = &st
	m.wsInvalid = st.Ws.Err != nil
	return m
}

// settingsEnv is the running application as Settings reads it.
func (m model) settingsEnv() settings.Env {
	return settings.Env{Machine: m.settingsMachine(), Apps: m.apps, Store: m.store, ConfigBroken: m.configBroken(),
		ConfigFeedback: m.configFeedback(), ConfigErr: m.configErr, Narrow: m.layout().w < mediumWidth}
}

// settingsMachine is the machine settings as the screen shows them, with a warning for a
// Harness or Plugin that will not run.
func (m model) settingsMachine() settings.Machine {
	mc := settings.Machine{Workspace: m.store.Root(), EditorSet: m.editor, EditorSource: m.editorSource, Downloads: m.downloads,
		Harness: m.harness, Plugin: m.plugin, ASCII: m.ascii, Inherit: m.inherit, FollowUpDays: m.followUpDays, Sources: m.sources,
		HarnessWarning: harnessWarning(m.harness), PluginWarning: pluginWarning(m.plugin),
		FollowUpWarning: m.configWarning(settings.KeyFollowUpDays)}
	mc.Editor = ResolveEditor(m.editor)
	mc.EditorAvailable = editorOnPath(mc.Editor)
	return mc
}

func (m model) settingsRows(group int) []settings.Row {
	return m.settings.GroupRows(group, m.settingsEnv())
}

// settingsHelpKeys is the keys Settings accepts. Application actions are absent, and so
// are the global keys.
func (m model) settingsHelpKeys() []helpKey {
	if m.settings != nil && m.settings.Edit != nil {
		return settingsKeys(m.settings.EditKeys())
	}
	env := m.settingsEnv()
	keys := []helpKey{}
	if m.settings != nil {
		keys = append(keys, settingsKeys(m.settings.RowKeys(env))...)
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
	if m.settings != nil {
		keys = append(keys, settingsKeys(m.settings.NavKeys(env))...)
	}
	if m.err != nil {
		keys = append(keys, help(groupNav, "esc", "clear error").at(prioPinned))
	} else if m.back != nil {
		keys = append(keys, help(groupNav, "esc", "back").at(prioPinned))
	}
	return keys
}

// settingsKeys declares Settings' keys in the one registry the key line, help and
// palette read.
func settingsKeys(ks []settings.Key) []helpKey {
	out := make([]helpKey, 0, len(ks))
	for _, k := range ks {
		group := groupAction
		if k.Nav {
			group = groupNav
		}
		h := help(group, k.Key, k.Desc)
		switch k.Prio {
		case settings.PrioCore:
			h = h.at(prioCore)
		case settings.PrioPinned:
			h = h.at(prioPinned)
		case settings.PrioHelpOnly:
			h = h.at(prioHelpOnly)
		}
		out = append(out, h)
	}
	return out
}

// settingsKey handles the keys Settings accepts. e hands the group's file to the Owner's
// editor and the screen re-reads when it exits; the rest go to the sub-model.
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
	st, in := m.settings.Update(k, m.settingsEnv())
	m.settings = &st
	next, cmd, _ := m.applySettings(in)
	return next, cmd
}

// editKey and editPaste type into the open inline field, which owns the keyboard.
func (m model) editKey(k tea.KeyPressMsg) model {
	st, in := m.settings.EditKey(k, m.settingsEnv())
	m.settings = &st
	m, _, _ = m.applySettings(in)
	return m
}

func (m model) editPaste(text string) model {
	st := m.settings.EditPaste(text)
	m.settings = &st
	return m
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

// applySettings applies one intent of the sub-model. err is a refused write whose pop-up
// keeps it under its field (Save.OnForm).
func (m model) applySettings(in settings.Intent) (model, tea.Cmd, error) {
	switch in := in.(type) {
	case settings.Save:
		return m.applySave(in)
	case settings.Restore:
		return m.undoSetting(in.Undo), nil, nil
	case settings.Open:
		m.overlay = &settingsPopup{in.Popup}
	case settings.Fail:
		m.err = in.Err
	case settings.Note:
		m.toast, m.toastKind = in.Text, toastNote
	case settings.CheckSource:
		return m.checkSource(in.Name)
	case settings.ShowTrack:
		m = m.showTrack(in.Track)
	}
	return m, nil, nil
}

// applySave writes, re-reads what the write changed, and reports the save on the feedback
// line, naming the file it went to. The undo is kept so u can put the old value back.
func (m model) applySave(s settings.Save) (model, tea.Cmd, error) {
	if err := m.writeSetting(s.Write); err != nil {
		if s.OnForm {
			return m, nil, err
		}
		m.err = err
		return m, nil, nil
	}
	_, keepsErr := s.Write.(settings.SetGate)
	if _, rule := s.Write.(settings.SetRule); rule {
		keepsErr = true
	}
	m = m.reloadSetting(s.Write, !keepsErr)
	switch s.Write.(type) {
	case settings.SetTracks, settings.SetMachine:
		if m.err != nil {
			return m, nil, nil // the re-read failed, so nothing is reported saved
		}
	}
	st, msg := m.settings.Saved(s, m.settingsEnv())
	m.settings = &st
	m.toast = msg + " · " + s.Write.File()
	return m, nil, nil
}

// undoSetting puts back the value the last Settings save replaced.
func (m model) undoSetting(u settings.Undo) model {
	if err := m.writeSetting(u.Restore); err != nil {
		m.err = err
	} else {
		_, tracks := u.Restore.(settings.SetTracks)
		_, machine := u.Restore.(settings.SetMachine)
		m = m.reloadSetting(u.Restore, tracks || machine)
	}
	st := m.settings.Undone()
	m.settings = &st
	if m.err == nil {
		m.toast, m.toastHint = "Undone · "+u.What, ""
	}
	return m
}

// writeSetting makes one Settings write: workspace.json through store, config.toml
// through config.
func (m model) writeSetting(w settings.Write) error {
	switch w := w.(type) {
	case settings.SetSearchArea:
		return m.store.SetSearchArea(w.Places, w.Remote)
	case settings.SetRule:
		return m.setRule(w.Key, w.Text)
	case settings.SetGate:
		return m.store.SetGate(w.Key, w.Text)
	case settings.SetTracks:
		return m.store.SetTracks(w.Tracks)
	case settings.SetSourceEnabled:
		return m.store.SetSourceEnabled(w.Name, w.On)
	case settings.AddSource:
		return m.store.AddSource(w.Source)
	case settings.RemoveSource:
		return m.store.RemoveSource(w.Name)
	case settings.UpdateSource:
		return m.store.UpdateSource(w.Name, w.Source)
	case settings.SetMachine:
		return m.patchConfig(w.Key, w.Value)
	}
	return fmt.Errorf("unknown Settings write %T", w)
}

// setRule writes a Rules row's value; empty text clears it.
func (m model) setRule(key, text string) error {
	if settings.IsVisa(key) {
		return m.store.SetVisa(key, text)
	}
	return m.store.SetSearchRule(key, text)
}

// reloadSetting re-reads what a write changed, so the change shows everywhere: Tracks
// rescan for the header filter, config.toml re-reads the machine settings (the help bar,
// Build and fetch availability, follow-up flags, Stats and the glyphs all follow), and the
// Rules re-read the rules the lists warn by.
func (m model) reloadSetting(w settings.Write, clearErr bool) model {
	if clearErr {
		m.err = nil
	}
	switch w.(type) {
	case settings.SetTracks:
		m.rescanAll()
	case settings.SetMachine:
		return m.loadMachine()
	case settings.SetRule:
		m = m.loadSettings()
		m.loadRules()
		return m
	}
	return m.loadSettings()
}

// settingsPopup is a Settings pop-up over the frame: a form drawn as every form is, or
// the rename review and the refusal the sub-model draws.
type settingsPopup struct{ p *settings.Popup }

func (sp *settingsPopup) Keys() [][2]string { return sp.p.Keys() }

func (sp *settingsPopup) View(m *model, p pal) string {
	if sp.p.Form != nil {
		return m.formBox(p, sp.p.Form, popSmall)
	}
	return sp.p.View(m.settingsLook(p))
}

func (sp *settingsPopup) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	next, in := sp.p.Update(msg, *m.settings, m.settingsEnv())
	if r, ok := in.(settings.RenameTrack); ok {
		return m.renameTrack(sp.p, r), nil
	}
	nm, cmd, err := m.applySettings(in)
	*m = nm
	if err != nil {
		sp.p.Form.Focused().Err = err.Error()
		return sp, cmd
	}
	if next == nil {
		return nil, cmd
	}
	return &settingsPopup{next}, cmd
}

// renameTrack runs a confirmed rename, then re-reads so the header filter, Applications
// and Settings follow. Skipped files keep the pop-up open to report them.
func (m *model) renameTrack(p *settings.Popup, r settings.RenameTrack) mode {
	res, err := m.store.RenameTrack(r.Old, r.New)
	m.undo = nil // a rename is not a move: u must not reach back across it
	m.rescanAll()
	*m = m.loadSettings()
	if err != nil {
		m.err = err
		return nil
	}
	next, msg := p.Renamed(res)
	if next != nil {
		return &settingsPopup{next}
	}
	m.toast = msg + " · " + settingsFile
	st := m.settings.Undone()
	m.settings = &st
	return nil
}

// checkSource runs the Plugin's Sweep against one Source in the background
// (sweep.py --test-source) and reports what it found. With no Plugin set it says where to
// set one.
func (m model) checkSource(name string) (model, tea.Cmd, error) {
	if m.plugin == "" {
		m.err = errNoPlugin
		return m, nil, nil
	}
	m.toast, m.toastHint, m.toastKind = "Checking "+name+"…", "", toastNote
	args := []string{filepath.Join(m.plugin, "scripts", "sweep.py"), dataFlag, m.store.Root(), "--test-source", name}
	return m, m.runCaptured("python3", args, func(r CapturedResult) tea.Msg {
		return sourceChecked{name: name, result: r}
	}), nil
}

// showTrack leaves Settings for the list that holds most of a Track's Applications,
// filtered to that Track: the Pipeline screen's All view, Drafts or Leads.
func (m model) showTrack(track string) model {
	type place struct{ screen, table int }
	count := map[place]int{}
	best, most := place{screen: screenIndex(screenPipeline)}, 0
	for _, a := range m.apps {
		if a.Track != track || a.Malformed != nil {
			continue
		}
		name, table := appLocation(a)
		if name == screenPipeline {
			table = allView
		}
		at := place{screen: screenIndex(name)}
		at.table = max(0, slices.Index(screenDefs[at.screen].tables, table))
		if count[at]++; count[at] > most {
			best, most = at, count[at]
		}
	}
	m.back, m.detail = nil, nil
	m.screen, m.table, m.cursor = best.screen, best.table, 0
	m.track = slices.Index(m.filterTracks(), track) + 1
	return m
}

// settingsLook is the palette, measures and helpers Settings draws with.
func (m model) settingsLook(p pal) settings.Look {
	return settings.Look{Palette: p.Palette, Edge: edge, Inset: inset, RightMargin: rightMargin, Gutter: gutter,
		Marker: profileMarkerWidth, AboutGutter: profileGutter, AboutDivider: profileDivider,
		PopWidth: popSmall, PopInset: popInset, ErrorMark: errorMark,
		Wrap: wrapWords, KeepTogether: keepTogether, Divider: p.Divider, ClipLines: p.ClipLines, ContextRow: contextRow,
		Popup: func(title string, warn bool, body, hints string) string {
			return popup(p, title, warn, popSmall, m.viewWidth(), body, hints)
		}}
}

// settingsView is the Settings screen in the same frame Profile uses. At wide sizes the
// focused row's about text has a pane of its own beside the rows.
func (m model) settingsView(p pal) string {
	w, h := m.viewWidth(), m.profileBodyHeight()
	split := 0
	if m.settingsAboutShown() {
		split = m.settingsRowsWidth(w)
	}
	return m.frame(p, []string{m.settingsContextRow(p, w)}, m.settingsBody(p, w, h, split), split, focusLeft, m.statusRow(p, m.keyLine(p, w)))
}

// settingsAboutShown reports whether the about pane sits beside the rows: at wide sizes,
// with a valid workspace.json.
func (m model) settingsAboutShown() bool {
	return m.layout().wide() && m.settings != nil && m.settings.Ws.Err == nil
}

// settingsRowsWidth is the rows pane beside the about pane and its divider: the list's
// width on the list screens, so the divider stays in one column moving between them and
// the about pane is as wide as the preview.
func (m model) settingsRowsWidth(w int) int {
	if l := m.layout(); l.detailW > 0 {
		return l.listW
	}
	return w
}

// settingsContextRow is the heading and the section tabs.
func (m model) settingsContextRow(p pal, w int) string {
	if m.settings == nil {
		return strings.Repeat(" ", inset) + p.Heading("SETTINGS")
	}
	return m.settings.Context(m.settingsLook(p), w, m.withChip(p, ""))
}

// settingsBody is the settings of the selected section, or the invalid-file notice; with
// rowsW > 0 the about pane sits beside the rows.
func (m model) settingsBody(p pal, w, h, rowsW int) string {
	if m.settings == nil {
		return lipgloss.Place(w, h, lipgloss.Left, lipgloss.Top, "")
	}
	return m.settings.Body(m.settingsLook(p), m.settingsEnv(), w, h, rowsW)
}

// settingsRowsView lists the selected group's settings, as the screen draws them.
func (m model) settingsRowsView(p pal, width, h int, aboutBelow bool) string {
	return m.settings.RowsView(m.settingsLook(p), m.settingsEnv(), width, h, aboutBelow)
}

// patchConfig writes one config.toml key.
func (m model) patchConfig(key string, value any) error {
	if m.configHome == "" {
		return errNoConfigHome
	}
	return config.Patch(config.Path(m.configHome), key, value)
}
