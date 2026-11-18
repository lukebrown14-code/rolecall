package settings

import (
	"fmt"
	"strconv"

	"github.com/lukebrown14-code/rolecall/internal/config"
)

// config.toml keys of the editable machine rows.
const (
	KeyFollowUpDays = "follow_up_days"
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
		{group: GroupTracking, key: KeyFollowUpDays, label: LabelFollowUp},
	}
	for i, s := range []machineSetting{
		{group: GroupMachine, key: keyEditor, label: LabelEditor},
		{group: GroupMachine, key: keyDownloads, label: LabelDownloads},
		{group: GroupMachine, key: keyHarness, label: LabelHarness},
		{group: GroupMachine, key: keyPlugin, label: LabelPlugin},
		{group: GroupMachine, key: "ascii", label: LabelASCII, toggle: true},
		{group: GroupMachine, key: keyInherit, label: LabelInherit, toggle: true},
	} {
		s.row = i + 1
		rows = append(rows, s)
	}
	return rows
}

// rowSetting is the editable setting on row of a group that has rows rows. Follow-up days
// is the last Tracking row, so it moves down as Tracks are added.
func rowSetting(group, row, rows int) (machineSetting, bool) {
	if group == GroupTracking {
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

// settingByKey is the editable setting with a config.toml key.
func settingByKey(key string) machineSetting {
	for _, s := range machineSettings {
		if s.key == key {
			return s
		}
	}
	return machineSetting{}
}

// savedText is a machine setting as config.toml sets it, as the Owner would type it: empty
// when the key is unset and the value is a default or comes from the environment.
func savedText(s machineSetting, mc Machine) string {
	switch s.key {
	case KeyFollowUpDays:
		if mc.Sources.FollowUpDays == config.SourceDefault {
			return ""
		}
		return strconv.Itoa(mc.FollowUpDays)
	case keyEditor:
		return mc.EditorSet
	case keyDownloads:
		if mc.Sources.Downloads != config.SourceConfig {
			return ""
		}
		return mc.Downloads
	case keyHarness:
		return mc.Harness
	case keyPlugin:
		if mc.Sources.Plugin != config.SourceConfig {
			return ""
		}
		return mc.Plugin
	case keyInherit:
		return YesNo(mc.Inherit)
	}
	return YesNo(mc.ASCII)
}

// effectiveText is the value in effect when config.toml leaves a setting unset: the
// placeholder an empty field shows.
func effectiveText(s machineSetting, mc Machine) string {
	switch s.key {
	case KeyFollowUpDays:
		return strconv.Itoa(mc.FollowUpDays)
	case keyEditor:
		return mc.Editor
	case keyDownloads:
		return mc.Downloads
	case keyHarness:
		return ExampleHarness
	case keyPlugin:
		if mc.Plugin != "" {
			return mc.Plugin
		}
		return ExamplePlugin
	}
	return ""
}

// fallbackText is what an unset machine setting falls back to, as its undo names it, so
// the toast matches the row: "$VISUAL (nvim)", "the default (14)", or "not set".
func fallbackText(s machineSetting, mc Machine) string {
	switch s.key {
	case keyEditor:
		if src := mc.EditorSource; src == config.SourceVisual || src == config.SourceEditor {
			return src + " (" + mc.Editor + ")"
		}
	case KeyFollowUpDays:
		return "the default (" + strconv.Itoa(mc.FollowUpDays) + ")"
	case keyDownloads:
		if mc.Downloads != "" {
			return "the default (" + mc.Downloads + ")"
		}
	case keyPlugin:
		if mc.Plugin != "" {
			return mc.Sources.Plugin + " (" + mc.Plugin + ")"
		}
	}
	return "not set"
}

// typedValue is a typed setting as config.toml holds it.
func typedValue(s machineSetting, text string) any {
	switch {
	case s.key == KeyFollowUpDays:
		n, _ := strconv.Atoi(text)
		return n
	case s.toggle:
		return text == YesNo(true)
	}
	return text
}

// activate handles Enter on a row: a text setting opens its inline field, a Yes/No flips
// and saves. A set value opens selected, so typing replaces it; an unset one opens empty,
// its default shown as a placeholder, so typing never appends to a default.
func (m Model) activate(env Env) (Model, Intent) {
	s, ok := rowSetting(m.Group, m.Row, len(m.GroupRows(m.Group, env)))
	if !ok {
		return m, nil
	}
	if env.ConfigBroken {
		// A save would patch a file that does not parse; fix it first.
		return m, Fail{Err: env.ConfigFeedback}
	}
	if s.toggle {
		return m, saveMachine(s, YesNo(savedText(s, env.Machine) != YesNo(true)), env.Machine)
	}
	text := savedText(s, env.Machine)
	m.Edit = &Edit{Key: s.key, Buf: text, Orig: text, Selected: text != "", Placeholder: effectiveText(s, env.Machine)}
	return m, nil
}

// saveMachine writes one machine setting to config.toml at once; the application re-reads
// the file, so the change shows everywhere. u puts the old value back. An empty text value
// clears the key, so the default applies again. Saving the value already set writes nothing.
func saveMachine(s machineSetting, text string, mc Machine) Intent {
	was := savedText(s, mc)
	if text == was {
		return nil
	}
	shown := was
	if shown == "" {
		shown = fallbackText(s, mc) // named before the save changes what is in effect
	}
	msg := fmt.Sprintf("Saved %s: %s", s.label, text)
	if text == "" {
		msg = "Cleared " + s.label + ": the default applies"
	}
	// config.toml cannot unset a number, so an unset follow-up goes back to the default it had.
	old := typedValue(s, was)
	if s.key == KeyFollowUpDays && was == "" {
		old = mc.FollowUpDays
	}
	return Save{Write: SetMachine{Key: s.key, Value: typedValue(s, text)}, Msg: msg,
		Undo: &Undo{What: s.label + " back to " + shown, Restore: SetMachine{Key: s.key, Value: old}}}
}

// machineKeys is Enter on an editable config.toml row.
func (m Model) machineKeys(env Env) []Key {
	if m.Ws.Err != nil || !m.rowsFocused() || env.ConfigBroken {
		return nil
	}
	s, ok := rowSetting(m.Group, m.Row, len(m.GroupRows(m.Group, env)))
	switch {
	case !ok:
		return nil
	case s.toggle:
		return []Key{{Key: "enter", Desc: "toggle", Prio: PrioCore}}
	}
	return []Key{{Key: "enter", Desc: descEdit, Prio: PrioCore}}
}

// FileOf is the file a row's value is kept in.
func (m Model) FileOf(group, row int, env Env) string {
	if _, ok := rowSetting(group, row, len(m.GroupRows(group, env))); ok || group == GroupMachine {
		return configFile
	}
	return settingsFile
}
