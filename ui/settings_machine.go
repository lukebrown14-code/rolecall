package ui

import (
	"errors"
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

// Follow-up days bounds, as the issue states them.
const (
	minFollowUpDays = 1
	maxFollowUpDays = 90
)

var errFollowUpDays = errors.New("Follow-up days must be a whole number from 1 to 90")

// machineSetting is an editable config.toml row.
type machineSetting struct {
	group, row int
	key        string // config.toml key
	toggle     bool   // Yes/No, flipped by Enter
}

// machineSettings are the editable rows. Workspace is shown but absent: it is never written.
var machineSettings = []machineSetting{
	{2, 1, keyFollowUpDays, false},
	{3, 1, "editor", false},
	{3, 2, "downloads", false},
	{3, 3, "harness", false},
	{3, 4, "plugin", false},
	{3, 5, "ascii", true},
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

// settingsRowsShown is the group's rows with the row being edited showing its input.
func (m model) settingsRowsShown(group int) []settingRow {
	rows := m.settingsRows(group)
	if st := m.settings; st.editing && st.group == group && st.row < len(rows) {
		rows[st.row].value = st.input + "▏"
	}
	return rows
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

// activateSetting handles Enter on a row: a text setting opens for editing, a Yes/No flips.
func (m model) activateSetting() (tea.Model, tea.Cmd) {
	st := *m.settings
	s, ok := m.machineSettingAt(st.group, st.row)
	if !ok {
		return m, nil
	}
	if s.toggle {
		return m.saveMachine(s, !m.ascii), nil
	}
	st.editing, st.input = true, m.settingValue(s)
	m.settings = &st
	return m, nil
}

// settingsEditKey runs the input of the row being edited. Nothing is written until Enter,
// and a refused value leaves the input open so it can be fixed.
func (m model) settingsEditKey(k tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	st := *m.settings
	switch k.String() {
	case "esc":
		st.editing, st.input = false, ""
		m.err = nil
	case keyBackspace:
		if r := []rune(st.input); len(r) > 0 {
			st.input = string(r[:len(r)-1])
		}
	case "enter":
		s, _ := m.machineSettingAt(st.group, st.row)
		text := strings.TrimSpace(st.input)
		var value any = text
		if s.key == keyFollowUpDays {
			days, err := strconv.Atoi(text)
			if err != nil || days < minFollowUpDays || days > maxFollowUpDays {
				m.err = errFollowUpDays
				return m, nil
			}
			value = days
		}
		st.editing, st.input = false, ""
		m.settings = &st
		return m.saveMachine(s, value), nil
	default:
		st.input += k.Text
	}
	m.settings = &st
	return m, nil
}

// saveMachine patches config.toml and re-reads it so the change shows at once: the help
// bar, Build and fetch availability, follow-up flags, Stats and the glyphs all follow.
func (m model) saveMachine(s machineSetting, value any) model {
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
	m.followUpDays, m.ascii = c.FollowUpDays, c.ASCII
	return m
}

// machineHelpKeys is Enter on an editable config.toml row.
func (m model) machineHelpKeys() []helpKey {
	if m.settings == nil || m.settings.ws.Err != nil || m.settings.focus != settingsRows {
		return nil
	}
	s, ok := m.machineSettingAt(m.settings.group, m.settings.row)
	switch {
	case !ok:
		return nil
	case s.toggle:
		return []helpKey{help(groupAction, "enter", "toggle")}
	}
	return []helpKey{help(groupAction, "enter", "edit")}
}

// settingsEditing reports whether a Settings row is open for typing.
func (m model) settingsEditing() bool {
	return m.screen == settingsScreen && m.settings != nil && m.settings.editing
}
