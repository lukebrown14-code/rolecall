package ui

import (
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	xansi "github.com/charmbracelet/x/ansi"
)

// settingsMode is the `,` overlay: the effective settings, read-only. It is an
// overlay, not a screen, so closing it leaves the screen, table, cursor and filter as
// they were.
type settingsMode struct{}

func (sm *settingsMode) Keys() [][2]string { return [][2]string{{"esc", hintClose}} }

func (sm *settingsMode) boxed() bool { return true }

func (sm *settingsMode) View(m *model, p pal) string { return m.settingsBox(p, sm) }

func (sm *settingsMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok && k.String() == "esc" {
		return nil, nil
	}
	return sm, nil
}

// editorLine is the Editor setting: the effective command and where it came from.
func (m model) editorLine() string {
	line := ResolveEditor(m.editor)
	if m.editorSource != "" {
		line += " (" + m.editorSource + ")"
	}
	return line
}

const minSettingsWrap = 20

func withSource(value, source string) string {
	if source == "" {
		return value
	}
	return value + " (" + source + ")"
}

func orNotSet(v string) string {
	if v == "" {
		return "not set"
	}
	return v
}

func (m model) settingsBox(p pal, sm *settingsMode) string {
	rows := [][2]string{{"Editor", m.editorLine()},
		{"Workspace", withSource(displayPath(m.store.Root()), m.sources.Workspace)},
		{"Follow-up days", withSource(strconv.Itoa(m.followUpDays), m.sources.FollowUpDays)},
		{"Downloads", withSource(displayPath(m.downloads), m.sources.Downloads)},
		{"Plugin", withSource(orNotSet(displayPath(m.plugin)), m.sources.Plugin)},
		{"Harness", withSource(orNotSet(m.harness), m.sources.Harness)}}
	// Long values wrap, indented under their label, so none is cut on a narrow terminal.
	avail := max(minSettingsWrap, popWidth(popForm, m.viewWidth())-popEdges-gutter)
	var b strings.Builder
	for _, r := range rows {
		label := r[0] + ": "
		wrapped := strings.Split(xansi.Hardwrap(r[1], max(1, avail-len(label)), true), "\n")
		for i, l := range wrapped {
			if i == 0 {
				b.WriteString(p.text(p.sub).Render(label))
			} else {
				b.WriteString(strings.Repeat(" ", len(label)))
			}
			b.WriteString(p.text(p.fg).Bold(true).Render(l) + "\n")
		}
	}
	return popup(p, "Settings", false, popForm, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"), p.keys(sm.Keys()))
}
