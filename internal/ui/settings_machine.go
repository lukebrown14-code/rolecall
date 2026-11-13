package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// WithConfigHome says where config.toml lives (under home), so Settings can write it and
// re-read it. Without it the machine settings are shown but cannot be saved.
func WithConfigHome(home string) Option { return func(m *model) { m.configHome = home } }

// WithASCII turns ASCII glyphs on, as the ascii key in config.toml does.
func WithASCII(on bool) Option { return func(m *model) { m.ascii = on } }

const (
	keyFollowUpDays = "follow_up_days"
	keyEditor       = "editor"
)

// machineSetting is an editable config.toml row.
type machineSetting struct {
	group, row int
	key        string // config.toml key
	label      string // how the row reads in Settings, for the save message
	toggle     bool   // Yes/No, flipped by Enter
}

// machineSettings are the editable rows. Workspace is shown but absent: it is never
// written, so the machine rows start at row one. The rows are numbered from their order.
var machineSettings = machineSettingsList()

func machineSettingsList() []machineSetting {
	rows := []machineSetting{
		{group: settings.GroupTracking, key: keyFollowUpDays, label: labelFollowUp},
	}
	for i, s := range []machineSetting{
		{group: settings.GroupMachine, key: keyEditor, label: labelEditor},
		{group: settings.GroupMachine, key: "downloads", label: labelDownloads},
		{group: settings.GroupMachine, key: "harness", label: labelHarness},
		{group: settings.GroupMachine, key: "plugin", label: labelPlugin},
		{group: settings.GroupMachine, key: "ascii", label: labelASCII, toggle: true},
	} {
		s.row = i + 1
		rows = append(rows, s)
	}
	return rows
}

func yesNo(on bool) string { return settings.YesNo(on) }

// rowSetting is the editable setting on row of a group that has rows rows. Follow-up days
// is the last Tracking row, so it moves down as Tracks are added.
func (m model) rowSetting(group, row, rows int) (machineSetting, bool) {
	if group == settings.GroupTracking {
		if row == rows-1 {
			return machineSettings[0], true
		}
		return machineSetting{}, false
	}
	for _, s := range machineSettings[1:] {
		if s.group == group && s.row == row {
			return s, true
		}
	}
	return machineSetting{}, false
}

// savedText is a machine setting as saved, as the Owner would type it.
func (m model) savedText(s machineSetting) string {
	switch s.key {
	case keyFollowUpDays:
		return strconv.Itoa(m.followUpDays)
	case keyEditor:
		return m.editor
	case "downloads":
		return m.downloads
	case "harness":
		return m.harness
	case "plugin":
		return m.plugin
	}
	return yesNo(m.ascii)
}

// shownText is a setting with any unsaved edit laid over it: what the field starts from.
// An unset Editor still resolves to a default, so that is what the Owner sees and edits.
func (m model) shownText(s machineSetting) string {
	v, ok := m.settings.Pending[s.key]
	if !ok {
		v = m.savedText(s)
	}
	if s.key == keyEditor && strings.TrimSpace(v) == "" {
		v = ResolveEditor(m.editor)
	}
	return v
}

// typedValue is a typed setting as config.toml holds it.
func typedValue(s machineSetting, text string) any {
	switch {
	case s.key == keyFollowUpDays:
		n, _ := strconv.Atoi(text)
		return n
	case s.toggle:
		return text == yesNo(true)
	}
	return text
}

// setPending records an edit, or drops it when it puts the saved value back.
func (m model) setPending(s machineSetting, text string) model {
	st := *m.settings
	if text == m.savedText(s) {
		st = st.Without(s.key)
	} else {
		st = st.With(s.key, text)
	}
	m.settings = &st
	return m
}

// activateSetting handles Enter on a row: a text setting opens its inline field, a Yes/No flips.
func (m model) activateSetting() (tea.Model, tea.Cmd) {
	st := *m.settings
	s, ok := m.rowSetting(st.Group, st.Row, len(m.settingsRows(st.Group)))
	if !ok {
		return m, nil
	}
	if s.toggle {
		return m.setPending(s, yesNo(m.shownText(s) != yesNo(true))), nil
	}
	st.Edit = &settings.Edit{Key: s.key, Buf: m.shownText(s)}
	m.settings = &st
	return m, nil
}

// settingByKey is the editable setting with a config.toml key.
func settingByKey(key string) machineSetting {
	for _, s := range machineSettings {
		if s.key == key {
			return s
		}
	}
	return machineSetting{}
}

// editKey types into the open inline field: enter keeps the edit as unsaved, esc drops it.
func (m model) editKey(k tea.KeyPressMsg) model {
	st := *m.settings
	e := *st.Edit
	s := settingByKey(e.Key)
	switch k.String() {
	case "esc":
		st.Edit = nil
	case "enter":
		text := strings.TrimSpace(e.Buf)
		if s.key == keyFollowUpDays {
			if err := validFollowUpDays(text); err != nil {
				e.Err = err.Error()
				st.Edit = &e
				break
			}
		}
		st.Edit = nil
		m.settings = &st
		if text == m.shownText(s) {
			return m
		}
		return m.setPending(s, text)
	case "backspace":
		if r := []rune(e.Buf); len(r) > 0 {
			e.Buf = string(r[:len(r)-1])
		}
		e.Err, st.Edit = "", &e
	case "ctrl+u":
		e.Buf, e.Err, st.Edit = "", "", &e
	default:
		if k.Text != "" {
			e.Buf, e.Err, st.Edit = e.Buf+k.Text, "", &e
		}
	}
	m.settings = &st
	return m
}

// editPaste types pasted text into the open inline field, on one line.
func (m model) editPaste(text string) model {
	st := *m.settings
	e := *st.Edit
	e.Buf, e.Err = e.Buf+strings.Join(strings.Fields(text), " "), ""
	st.Edit = &e
	m.settings = &st
	return m
}

// saveSettings is ctrl+s: every unsaved edit goes to config.toml and the file is re-read, so
// the change shows at once: the help bar, Build and fetch availability, follow-up flags,
// Stats and the glyphs all follow. u puts the old values back.
func (m model) saveSettings() model {
	if m.settings == nil || m.settings.Dirty() == 0 {
		return m
	}
	type change struct {
		s        machineSetting
		new, old any
	}
	var changes []change
	for _, s := range machineSettings {
		if text, ok := m.settings.Pending[s.key]; ok {
			changes = append(changes, change{s, typedValue(s, text), typedValue(s, m.savedText(s))})
		}
	}
	for _, c := range changes {
		if m = m.patchMachine(c.s, c.new); m.err != nil {
			return m
		}
	}
	msg, what := "Saved "+plural(len(changes), "change", "changes"), "Settings back"
	if len(changes) == 1 {
		c := changes[0]
		shown := machineValue(c.old)
		if shown == "" {
			shown = "not set"
		}
		msg, what = fmt.Sprintf("Saved %s: %s", c.s.label, machineValue(c.new)), c.s.label+" back to "+shown
	}
	m = m.saved(msg, configFile, &settingsUndo{what: what, restore: func(m model) model {
		for _, c := range changes {
			if m = m.patchMachine(c.s, c.old); m.err != nil {
				break
			}
		}
		return m
	}})
	st := *m.settings
	st.Pending, st.Edit = nil, nil
	m.settings = &st
	return m
}

// revertSettings is ctrl+z: the field under the cursor goes back to its saved value, or
// every field does when the cursor is on the header.
func (m model) revertSettings() model {
	st := *m.settings
	if st.Header {
		st.Pending = nil
	} else if s, ok := m.rowSetting(st.Group, st.Row, len(m.settingsRows(st.Group))); ok {
		st = st.Without(s.key)
	}
	m.settings = &st
	return m
}

// deferredNote says which file this section's edits go to, and when. Search, Documents and
// the Tracks write through their own pop-ups, so only the sections with typed fields say it.
func (m model) deferredNote() string {
	switch m.settings.Group {
	case settings.GroupMachine:
		return "Changes save to " + configFile + " — nothing is written until ctrl+s."
	case settings.GroupTracking:
		return "Follow-up changes save to " + configFile + " — nothing is written until ctrl+s."
	}
	return ""
}

// leaveMode asks what to do with unsaved Settings changes before the Owner goes elsewhere.
type leaveMode struct{ then func(model) model }

// leaveSettings runs then at once, or after the Owner has saved or discarded the unsaved
// changes; cancelling stays in Settings.
func (m model) leaveSettings(then func(model) model) model {
	if m.settings == nil || m.settings.Dirty() == 0 {
		return then(m)
	}
	m.overlay = &leaveMode{then: then}
	return m
}

func (lm *leaveMode) Keys() [][2]string {
	return [][2]string{{"s", "save"}, {"d", "discard"}, {"esc", "cancel"}}
}

func (lm *leaveMode) View(m *model, p pal) string {
	body := p.Text(p.Fg).Bold(true).Render(plural(m.settings.Dirty(), "unsaved change", "unsaved changes")) + "\n\n" +
		p.Text(p.Sub).Render("Save them to "+configFile+" before leaving Settings?")
	return popup(p, "Unsaved changes", true, popSmall, m.viewWidth(), body, p.Keys(lm.Keys()))
}

func (lm *leaveMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	k, ok := msg.(tea.KeyPressMsg)
	if !ok {
		return lm, nil
	}
	switch k.String() {
	case "s":
		*m = m.saveSettings()
		if m.err != nil {
			return nil, nil
		}
	case "d":
		st := *m.settings
		st.Pending, st.Edit = nil, nil
		m.settings = &st
	case "esc":
		return nil, nil
	default:
		return lm, nil
	}
	*m = lm.then(*m)
	return nil, nil
}

// patchMachine writes one config.toml key and re-reads the file.
func (m model) patchMachine(s machineSetting, value any) model {
	if m.configHome == "" {
		m.err = errors.New("no config.toml to save to")
		return m
	}
	if err := config.Patch(config.Path(m.configHome), s.key, value); err != nil {
		m.err = err
		return m
	}
	m.err = nil
	return m.loadMachine()
}

// machineValue is a saved value as the feedback line reads it.
func machineValue(value any) string {
	if on, ok := value.(bool); ok {
		return yesNo(on)
	}
	return fmt.Sprint(value)
}

// loadMachine re-reads config.toml into the model.
func (m model) loadMachine() model {
	c, err := config.Load(m.configHome, m.store.Root())
	if err != nil {
		m.err = err
		return m
	}
	m.editor, m.downloads, m.harness, m.plugin = c.Editor, c.Downloads, c.Harness, c.Plugin
	m.followUpDays, m.ascii = c.FollowUpDays, c.ASCII
	m.editorSource = c.EditorSource
	// The Workspace is fixed for the session (the store is open), so its source stays.
	m.sources = SettingSources{Workspace: m.sources.Workspace, FollowUpDays: c.FollowUpDaysSource,
		Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource}
	return m
}

// machineHelpKeys is Enter on an editable config.toml row.
func (m model) machineHelpKeys() []helpKey {
	if m.settings == nil || m.settings.Ws.Err != nil || !m.settingsRowsFocused() {
		return nil
	}
	s, ok := m.rowSetting(m.settings.Group, m.settings.Row, len(m.settingsRows(m.settings.Group)))
	switch {
	case !ok:
		return nil
	case s.toggle:
		return []helpKey{help(groupAction, "enter", "toggle").at(prioCore)}
	}
	return []helpKey{help(groupAction, "enter", "edit").at(prioCore)}
}

var validFollowUpDays = settings.ValidFollowUpDays
