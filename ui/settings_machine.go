package ui

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/config"
)

// WithConfigHome says where config.toml lives (under home), so Settings can write it and
// re-read it. Without it the machine settings are shown but cannot be saved.
func WithConfigHome(home string) Option { return func(m *model) { m.configHome = home } }

// WithASCII turns ASCII glyphs on, as the ascii key in config.toml does.
func WithASCII(on bool) Option { return func(m *model) { m.ascii = on } }

const (
	keyFollowUpDays = "follow_up_days"
	keyBackspace    = "backspace"
)

// groupMachine is the Settings group that holds the config.toml rows.
const groupMachine = 3

// Follow-up days bounds, as the issue states them.
const (
	minFollowUpDays = 1
	maxFollowUpDays = 90
)

var errFollowUpDays = errors.New("Follow-up must be a whole number of days, 1 to 90")

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
		{group: groupTracking, key: keyFollowUpDays, label: labelFollowUp},
	}
	for i, s := range []machineSetting{
		{group: groupMachine, key: "editor", label: labelEditor},
		{group: groupMachine, key: "downloads", label: labelDownloads},
		{group: groupMachine, key: "harness", label: labelHarness},
		{group: groupMachine, key: "plugin", label: labelPlugin},
		{group: groupMachine, key: "ascii", label: labelASCII, toggle: true},
	} {
		s.row = i + 1
		rows = append(rows, s)
	}
	return rows
}

// machineSettingAt is the editable setting on a row. Follow-up days is the last Tracking
// row, so it moves down as Tracks are added.
func (m model) machineSettingAt(group, row int) (machineSetting, bool) {
	if group == groupTracking && row == len(m.trackingRows())-1 {
		return machineSettings[0], true
	}
	for _, s := range machineSettings[1:] {
		if s.group == group && s.row == row {
			return s, true
		}
	}
	return machineSetting{}, false
}

func yesNo(on bool) string {
	if on {
		return "Yes"
	}
	return "No"
}

// machineExample is what to type into an unset machine setting.
func (m model) machineExample(s machineSetting) string {
	for _, r := range m.settingsRows(s.group) {
		if r.label == s.label {
			return r.example
		}
	}
	return ""
}

// settingValue is a machine setting as the Owner would type it.
func (m model) settingValue(s machineSetting) string {
	switch s.key {
	case keyFollowUpDays:
		return strconv.Itoa(m.followUpDays)
	case "editor":
		return m.editor
	case "downloads":
		return m.downloads
	case "harness":
		return m.harness
	case "plugin":
		return m.plugin
	}
	return ""
}

// validFollowUpDays is the follow-up rule, checked as the Owner leaves the field.
func validFollowUpDays(s string) error {
	days, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil || days < minFollowUpDays || days > maxFollowUpDays {
		return errFollowUpDays
	}
	return nil
}

// activateSetting handles Enter on a row: a text setting opens its pop-up, a Yes/No flips.
func (m model) activateSetting() (tea.Model, tea.Cmd) {
	st := *m.settings
	s, ok := m.machineSettingAt(st.group, st.row)
	if !ok {
		return m, nil
	}
	if s.toggle {
		return m.saveMachine(s, !m.ascii, m.ascii), nil
	}
	m.overlay = m.machinePopup(s)
	return m, nil
}

// machinePopup edits one config.toml setting. Nothing is written until it is saved.
func (m model) machinePopup(s machineSetting) mode {
	was := m.settingValue(s)
	fld := formField{label: s.label, value: was, placeholder: m.machineExample(s)}
	if s.key == keyFollowUpDays {
		fld.required, fld.unit, fld.note, fld.validate = true, "days", "1 to 90", validFollowUpDays
	}
	return &formPopup{form: newFormOf("Edit "+s.label, fld), width: popSmall, save: func(m *model, f *form) (mode, error) {
		text := strings.TrimSpace(f.value(s.label))
		var value, old any = text, was
		if s.key == keyFollowUpDays {
			if err := validFollowUpDays(text); err != nil {
				return nil, err
			}
			value, _ = strconv.Atoi(text)
			old, _ = strconv.Atoi(was)
		}
		next := m.saveMachine(s, value, old)
		if next.err != nil {
			return nil, next.err
		}
		*m = next
		return nil, nil
	}}
}

// saveMachine patches config.toml and re-reads it so the change shows at once: the help
// bar, Build and fetch availability, follow-up flags, Stats and the glyphs all follow. It
// says what was saved, and where, on the feedback line, and u puts old back.
func (m model) saveMachine(s machineSetting, value, old any) model {
	if m = m.patchMachine(s, value); m.err != nil {
		return m
	}
	shown := machineValue(old)
	if shown == "" {
		shown = "not set"
	}
	return m.saved(fmt.Sprintf("Saved %s: %s", s.label, machineValue(value)), configFile, &settingsUndo{
		what:    s.label + " back to " + shown,
		restore: func(m model) model { return m.patchMachine(s, old) },
	})
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
	if m.settings == nil || m.settings.ws.Err != nil || !m.settingsRowsFocused() {
		return nil
	}
	s, ok := m.machineSettingAt(m.settings.group, m.settings.row)
	switch {
	case !ok:
		return nil
	case s.toggle:
		return []helpKey{help(groupAction, "enter", "toggle").at(prioCore)}
	}
	return []helpKey{help(groupAction, "enter", "edit").at(prioCore)}
}
