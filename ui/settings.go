package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
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
	editor := ResolveEditor(m.editor)
	line := editor
	if m.editorSource != "" {
		line += " (" + m.editorSource + ")"
	}
	if !EditorOnPath(editor) {
		line += " (not found on PATH)"
	}
	return line
}

func (m model) settingsBox(p pal, sm *settingsMode) string {
	rows := [][2]string{{"Editor", m.editorLine()}}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(p.text(p.sub).Render(r[0]+": ") + p.text(p.fg).Bold(true).Render(r[1]) + "\n")
	}
	return popup(p, "Settings", false, popForm, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"), p.keys(sm.Keys()))
}
