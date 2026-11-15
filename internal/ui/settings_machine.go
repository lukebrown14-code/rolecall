package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// WithConfigHome says where config.toml lives (under home), so Settings can write it and
// re-read it. Without it the machine settings are shown but cannot be saved.
func WithConfigHome(home string) Option { return func(m *model) { m.configHome = home } }

// WithInheritTerminal draws with the terminal's ANSI-16 colours and no background, as the
// inherit_terminal key in config.toml does.
func WithInheritTerminal(on bool) Option { return func(m *model) { m.inherit = on } }

// WithASCII turns ASCII glyphs on, as the ascii key in config.toml does.
func WithASCII(on bool) Option { return func(m *model) { m.ascii = on } }

const (
	keyFollowUpDays = "follow_up_days"
	keyEditor       = "editor"
	keyInherit      = "inherit_terminal"
	keyDownloads    = "downloads"
	keyHarness      = "harness"
	keyPlugin       = "plugin"
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
		{group: settings.GroupMachine, key: keyDownloads, label: labelDownloads},
		{group: settings.GroupMachine, key: keyHarness, label: labelHarness},
		{group: settings.GroupMachine, key: keyPlugin, label: labelPlugin},
		{group: settings.GroupMachine, key: "ascii", label: labelASCII, toggle: true},
		{group: settings.GroupMachine, key: keyInherit, label: labelInherit, toggle: true},
	} {
		s.row = i + 1
		rows = append(rows, s)
	}
	return rows
}

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

// savedText is a machine setting as config.toml sets it, as the Owner would type it: empty
// when the key is unset and the value is a default or comes from the environment.
func (m model) savedText(s machineSetting) string {
	switch s.key {
	case keyFollowUpDays:
		if m.sources.FollowUpDays == config.SourceDefault {
			return ""
		}
		return strconv.Itoa(m.followUpDays)
	case keyEditor:
		return m.editor
	case keyDownloads:
		if m.sources.Downloads != config.SourceConfig {
			return ""
		}
		return m.downloads
	case keyHarness:
		return m.harness
	case keyPlugin:
		if m.sources.Plugin != config.SourceConfig {
			return ""
		}
		return m.plugin
	case keyInherit:
		return settings.YesNo(m.inherit)
	}
	return settings.YesNo(m.ascii)
}

// effectiveText is the value in effect when config.toml leaves a setting unset: the
// placeholder an empty field shows.
func (m model) effectiveText(s machineSetting) string {
	switch s.key {
	case keyFollowUpDays:
		return strconv.Itoa(m.followUpDays)
	case keyEditor:
		return ResolveEditor(m.editor)
	case keyDownloads:
		return m.downloads
	case keyHarness:
		return settings.ExampleHarness
	case keyPlugin:
		if m.plugin != "" {
			return m.plugin
		}
		return settings.ExamplePlugin
	}
	return ""
}

// typedValue is a typed setting as config.toml holds it.
func typedValue(s machineSetting, text string) any {
	switch {
	case s.key == keyFollowUpDays:
		n, _ := strconv.Atoi(text)
		return n
	case s.toggle:
		return text == settings.YesNo(true)
	}
	return text
}

// activateSetting handles Enter on a row: a text setting opens its inline field, a Yes/No
// flips and saves. A set value opens selected, so typing replaces it; an unset one opens
// empty, its default shown as a placeholder, so typing never appends to a default.
func (m model) activateSetting() (tea.Model, tea.Cmd) {
	st := *m.settings
	s, ok := m.rowSetting(st.Group, st.Row, len(m.settingsRows(st.Group)))
	if !ok {
		return m, nil
	}
	if s.toggle {
		return m.saveMachine(s, settings.YesNo(m.savedText(s) != settings.YesNo(true))), nil
	}
	text := m.savedText(s)
	st.Edit = &settings.Edit{Key: s.key, Buf: text, Orig: text, Selected: text != "", Placeholder: m.effectiveText(s)}
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

// editKey types into the open inline field: enter saves the value at once, esc drops it.
// While a quit's "Discard changes?" is asked, y drops the edit and n or esc goes back to it.
func (m model) editKey(k tea.KeyPressMsg) model {
	st := *m.settings
	e := *st.Edit
	if e.Discarding {
		switch k.String() {
		case "y":
			st.Edit = nil
		case "n", "esc":
			e.Discarding = false
			st.Edit = &e
		}
		m.settings = &st
		return m
	}
	s := settingByKey(e.Key)
	switch k.String() {
	case "esc":
		st.Edit = nil
	case "enter":
		text := strings.TrimSpace(e.Buf)
		if text == strings.TrimSpace(e.Orig) {
			st.Edit = nil // nothing changed, so nothing is written
			break
		}
		if s.key == keyFollowUpDays {
			if err := validFollowUpDays(text); err != nil {
				e.Err, e.Selected = err.Error(), false
				st.Edit = &e
				break
			}
		}
		st.Edit = nil
		m.settings = &st
		return m.saveMachine(s, text)
	case "backspace":
		if r := []rune(e.Buf); e.Selected {
			e.Buf = ""
		} else if len(r) > 0 {
			e.Buf = string(r[:len(r)-1])
		}
		e.Err, e.Selected, st.Edit = "", false, &e
	case "ctrl+u":
		e.Buf, e.Err, e.Selected, st.Edit = "", "", false, &e
	case "left", "right", "end":
		e.Selected, st.Edit = false, &e // the selection drops and the cursor stays at the end
	default:
		if k.Text != "" {
			if e.Selected {
				e.Buf = ""
			}
			e.Buf, e.Err, e.Selected, st.Edit = e.Buf+k.Text, "", false, &e
		}
	}
	m.settings = &st
	return m
}

// editPaste types pasted text into the open inline field, on one line, replacing a
// selected opening value.
func (m model) editPaste(text string) model {
	st := *m.settings
	e := *st.Edit
	if e.Discarding {
		return m
	}
	if e.Selected {
		e.Buf = ""
	}
	e.Buf, e.Err, e.Selected = e.Buf+strings.Join(strings.Fields(text), " "), "", false
	st.Edit = &e
	m.settings = &st
	return m
}

// saveMachine writes one machine setting to config.toml at once and re-reads the file, so
// the change shows everywhere: the help bar, Build and fetch availability, follow-up flags,
// Stats and the glyphs all follow. u puts the old value back. An empty text value clears
// the key, so the default applies again.
func (m model) saveMachine(s machineSetting, text string) model {
	was := m.savedText(s)
	if text == was {
		return m
	}
	// config.toml cannot unset a number, so an unset follow-up goes back to the default it had.
	wasDays := m.followUpDays
	if m = m.patchMachine(s, typedValue(s, text)); m.err != nil {
		return m
	}
	msg := fmt.Sprintf("Saved %s: %s", s.label, text)
	if text == "" {
		msg = "Cleared " + s.label + ": the default applies"
	}
	shown := was
	if shown == "" {
		shown = "not set"
	}
	return m.saved(msg, configFile, &settingsUndo{what: s.label + " back to " + shown, restore: func(m model) model {
		old := typedValue(s, was)
		if s.key == keyFollowUpDays && was == "" {
			old = wasDays
		}
		return m.patchMachine(s, old)
	}})
}

// quitHeld is ctrl+c while the Owner has typed into a Settings field or pop-up, or the Add
// or Edit Application form: rather than
// drop the text, it asks "Discard changes?" as esc on a form does. held is false when there
// is nothing typed to lose, or the question is already up, and the quit goes ahead.
func (m model) quitHeld() (next model, held bool) {
	var f *form
	switch o := m.overlay.(type) {
	case *formPopup:
		f = o.form
	case *formMode: // the Add and Edit Application forms
		f = o.form
	}
	if f != nil {
		if f.Discarding || !f.Dirty() {
			return m, false
		}
		f.Discarding = true
		return m, true
	}
	if !m.fieldOpen() {
		return m, false
	}
	e := *m.settings.Edit
	if e.Discarding || !e.Changed() {
		return m, false
	}
	e.Discarding = true
	st := *m.settings
	st.Edit = &e
	m.settings = &st
	return m, true
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

// loadMachine re-reads config.toml into the model.
func (m model) loadMachine() model {
	c, err := config.Load(m.configHome, m.store.Root())
	if err != nil {
		m.err = err
		return m
	}
	m.editor, m.downloads, m.harness, m.plugin = c.Editor, c.Downloads, c.Harness, c.Plugin
	m.followUpDays, m.ascii, m.inherit = c.FollowUpDays, c.ASCII, c.InheritTerminal
	m.editorSource = c.EditorSource
	// The Workspace is fixed for the session (the store is open), so its source stays.
	m.sources = SettingSources{Workspace: m.sources.Workspace, FollowUpDays: c.FollowUpDaysSource,
		Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource}
	return m
}

// harnessWarning names a Harness whose program is not on PATH. It never blocks a save.
func harnessWarning(harness string) string {
	if strings.TrimSpace(harness) == "" || editorOnPath(harness) {
		return ""
	}
	return firstWord(harness) + " not found on PATH"
}

// pluginWarning names a Plugin folder that is missing or holds no skills/. It never
// blocks a save.
func pluginWarning(plugin string) string {
	if strings.TrimSpace(plugin) == "" {
		return ""
	}
	if info, err := os.Stat(plugin); err != nil || !info.IsDir() {
		return "folder not found"
	}
	if info, err := os.Stat(filepath.Join(plugin, "skills")); err != nil || !info.IsDir() {
		return "no skills/ folder in it"
	}
	return ""
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
