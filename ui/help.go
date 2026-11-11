package ui

import (
	"strings"

	"charm.land/bubbles/v2/key"
)

// helpGroup places a key in the table view's help layout.
type helpGroup int

const (
	groupTrack  helpGroup = iota // beside the track filter
	groupFilter                  // beside the text filter
	groupRow                     // actions on the selected row
	groupOther                   // everything else
)

type helpKey struct {
	key.Binding
	group helpGroup
}

func help(group helpGroup, k, desc string) helpKey {
	return helpKey{key.NewBinding(key.WithKeys(k), key.WithHelp(k, desc)), group}
}

func (k helpKey) text() string {
	h := k.Help()
	return h.Key + " " + h.Desc
}

// overlayOpen reports whether a form, popup, prompt or filter box owns the keyboard.
func (m model) overlayOpen() bool {
	return m.addForm != nil || m.duplicate != nil || m.moveForm != nil || len(m.options) > 0 ||
		m.confirm != nil || m.pendingDownload != nil || m.filtering
}

// helpKeys is the one key map behind the help bar and `?` full help: only the keys that do
// something in the current context. An open overlay owns the keyboard, so it offers its own keys
// and nothing from the table or detail view beneath it; prompts print their own choices.
func (m model) helpKeys() []helpKey {
	if m.overlayOpen() {
		var keys []helpKey
		switch {
		case m.addForm != nil:
			keys = append(keys, help(groupOther, "Esc", "cancel"))
		case m.moveForm != nil || len(m.options) > 0:
			keys = append(keys, help(groupOther, "Enter", "move"), help(groupOther, "Esc", "cancel"))
		case m.filtering:
			keys = append(keys, help(groupOther, "Enter", "apply"), help(groupOther, "Esc", "clear"))
		}
		return append(keys, help(groupOther, "Ctrl-C", "quit"))
	}
	a, onRow := m.current()
	var keys []helpKey
	if m.detail == nil {
		if len(m.tracks) > 0 {
			keys = append(keys, help(groupTrack, "t", "track"))
		}
		keys = append(keys, help(groupFilter, "/", "filter"), help(groupFilter, "Esc", "clear"))
	} else {
		keys = append(keys, help(groupOther, "←/→", "tabs"), help(groupOther, "↑↓", "scroll"))
	}
	if onRow {
		keys = append(keys, help(groupRow, "e", "edit"), help(groupRow, "m", "move"), help(groupRow, "u", "undo"))
		if m.detail == nil && (a.Status == "Lead" || a.Status == "Draft") {
			keys = append(keys, help(groupRow, "x", "dismiss"))
		}
		keys = append(keys, help(groupOther, "n", "notes"))
		if len(a.PDFs) > 0 {
			keys = append(keys, help(groupOther, "d", "download"))
		}
	}
	if m.detail != nil {
		return append(keys, help(groupOther, "Esc", "back"))
	}
	keys = append(keys, help(groupOther, "1/2/3", "screens"))
	if len(tables[m.screen]) > 1 {
		keys = append(keys, help(groupOther, "Tab/Shift-Tab", "tables"))
	}
	return append(keys,
		help(groupOther, "↑↓/j/k/g/G", "cursor"), help(groupOther, "a", "add"), help(groupOther, "r", "rescan"),
		help(groupOther, "?", "full help"), help(groupOther, "q/Ctrl-C", "quit"))
}

func joinKeys(keys []helpKey, sep string) string {
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k.text()
	}
	return strings.Join(parts, sep)
}

func inGroup(keys []helpKey, group helpGroup) []helpKey {
	var out []helpKey
	for _, k := range keys {
		if k.group == group {
			out = append(out, k)
		}
	}
	return out
}

// filterLine shows the track and text filters with their keys.
func (m model) filterLine() string {
	keys := m.helpKeys()
	var segments []string
	if len(m.tracks) > 0 {
		track := "All"
		if m.track > 0 {
			track = m.tracks[m.track-1]
		}
		segments = append(segments, strings.Join(append([]string{"Track: " + track}, keyTexts(inGroup(keys, groupTrack))...), " · "))
	}
	if filter := inGroup(keys, groupFilter); len(filter) > 0 {
		segments = append(segments, joinKeys(filter, " · "))
	}
	line := strings.Join(segments, "  ")
	if m.text != "" || m.filtering {
		line += " · Filter: " + m.text
	}
	return line
}

func keyTexts(keys []helpKey) []string {
	out := make([]string, len(keys))
	for i, k := range keys {
		out[i] = k.text()
	}
	return out
}

// rowHelp is the table view's line of actions on the selected row.
func (m model) rowHelp() string {
	if row := inGroup(m.helpKeys(), groupRow); len(row) > 0 {
		return joinKeys(row, " · ") + "\n"
	}
	return ""
}

// otherHelp is the rest of the table view's help, one key per line in full help.
func (m model) otherHelp() string {
	var b strings.Builder
	if m.full {
		b.WriteString("Full help\n")
	}
	for _, k := range inGroup(m.helpKeys(), groupOther) {
		b.WriteString(k.text() + "  ")
		if m.full {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// detailHelp is the detail view's help bar: every key valid there on one line.
func (m model) detailHelp() string {
	keys := m.helpKeys()
	return joinKeys(keys, " · ") + "\n"
}
