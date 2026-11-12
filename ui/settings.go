package ui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/config"
)

// settingsMode is the `,` overlay: the effective settings, read-only. It is an
// overlay, not a screen, so closing it leaves the screen, table, cursor and filter as
// they were.
type settingsMode struct{}

func (sm *settingsMode) Keys() [][2]string {
	return [][2]string{{"e", "edit config"}, {"esc", hintClose}}
}

// editorExamples is the help line's list of Editor values for config.toml.
const editorExamples = "Editor examples (editor in config.toml):\n  nvim, code --wait, zed --wait, hx, subl --wait"

func (sm *settingsMode) boxed() bool { return true }

func (sm *settingsMode) View(m *model, p pal) string { return m.settingsBox(p, sm) }

func (sm *settingsMode) Update(m *model, msg tea.Msg) (mode, tea.Cmd) {
	if k, ok := msg.(tea.KeyPressMsg); ok {
		switch k.String() {
		case "esc":
			return nil, nil
		case "e":
			return sm, m.editConfig()
		}
	}
	return sm, nil
}

// configEdited reports the Editor exiting on the config file.
type configEdited struct{ err error }

// reloadConfig applies the edited config: new values on success, the old ones kept and
// the error on the feedback line otherwise.
func (m *model) reloadConfig(done configEdited) {
	if done.err != nil {
		m.err = done.err
		return
	}
	if m.reloadCfg == nil {
		return
	}
	c, err := m.reloadCfg()
	if err != nil {
		m.err = err
		return
	}
	m.editor, m.editorSource = c.Editor, c.EditorSource
	m.followUpDays, m.downloads, m.plugin, m.harness = c.FollowUpDays, c.Downloads, c.Plugin, c.Harness
}

// WithConfigReload names the config file Settings edits and how to reload it.
func WithConfigReload(path string, reload func() (config.Config, error)) Option {
	return func(m *model) { m.configPath, m.reloadCfg = path, reload }
}

// editorLine is the Editor setting: the effective command and where it came from.
func (m model) editorLine() string {
	line := ResolveEditor(m.editor)
	if m.editorSource != "" {
		line += " (" + m.editorSource + ")"
	}
	return line
}

func (m model) settingsBox(p pal, sm *settingsMode) string {
	rows := [][2]string{{"Editor", m.editorLine()}}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(p.text(p.sub).Render(r[0]+": ") + p.text(p.fg).Bold(true).Render(r[1]) + "\n")
	}
	b.WriteString("\n" + p.text(p.dim).Render(editorExamples))
	return popup(p, "Settings", false, popForm, m.viewWidth(), strings.TrimSuffix(b.String(), "\n"), p.keys(sm.Keys()))
}
