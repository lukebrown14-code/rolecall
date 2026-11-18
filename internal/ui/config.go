package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/config"
)

// config.toml, the machine settings file: how the application loads it, reports its
// problems and checks the programs it names. Settings › This machine writes it through
// the adapter in settings.go.

// WithConfigHome says where config.toml lives (under home), so Settings can write it and
// re-read it. Without it the machine settings are shown but cannot be saved.
func WithConfigHome(home string) Option { return func(m *model) { m.configHome = home } }

// WithInheritTerminal draws with the terminal's ANSI-16 colours and no background, as the
// inherit_terminal key in config.toml does.
func WithInheritTerminal(on bool) Option { return func(m *model) { m.inherit = on } }

// WithASCII turns ASCII glyphs on, as the ascii key in config.toml does.
func WithASCII(on bool) Option { return func(m *model) { m.ascii = on } }

// WithConfigProblem opens the app with config.toml's load error (a *config.FileError, the
// app then running on defaults) and its warnings on the feedback line, and on This machine.
func WithConfigProblem(err error, warnings []config.Warning) Option {
	return func(m *model) {
		m.configErr, m.configWarnings = err, warnings
		if f := m.configFeedback(); f != nil {
			m.err = f
		}
	}
}

// errNoConfigHome refuses a machine setting's save when there is no config.toml to write.
var errNoConfigHome = errors.New("no config.toml to save to")

// configBroken reports a config.toml that does not parse: its rows show the last good
// values, Enter on them is refused, and e opens config.toml.
func (m model) configBroken() bool {
	var fe *config.FileError
	return errors.As(m.configErr, &fe)
}

// fixHint is how a Settings problem is fixed from any screen: e on a table screen edits
// the row under the cursor, so the key that opens the file is on the Settings screen.
// On Settings itself the feedback line drops the "press 6 then".
const fixHint = "6 then e to fix it"

// settingsBroken reports a workspace.json or config.toml that cannot be used as written.
func (m model) settingsBroken() bool { return m.wsInvalid || m.configBroken() }

// configFeedback is the feedback-line form of config.toml's problem: short enough to read
// at 100 columns, with where to go to fix it. nil when config.toml is fine.
func (m model) configFeedback() error {
	var fe *config.FileError
	switch {
	case errors.As(m.configErr, &fe):
		where := ""
		if w := fe.Where(); w != "" {
			where = " " + w
		}
		return fmt.Errorf("%s%s: %s · %s", configFile, where, fe.Reason, fixHint)
	case m.configErr != nil:
		return m.configErr
	case len(m.configWarnings) > 0:
		return errors.New(configFile + ": " + m.configWarnings[0].Text)
	}
	return nil
}

// configWarning is the warning Load gave for key, or "", for that setting's row.
func (m model) configWarning(key string) string {
	for _, w := range m.configWarnings {
		if w.Key == key {
			return w.Text
		}
	}
	return ""
}

// loadMachine re-reads config.toml into the model. On an error the values in use stay,
// and the error goes on the feedback line and This machine.
func (m model) loadMachine() model {
	c, err := config.Load(m.configHome, m.store.Root())
	m.configErr, m.configWarnings = err, c.Warnings
	if err != nil {
		m.configWarnings = nil
		m.err = m.configFeedback()
		return m
	}
	if f := m.configFeedback(); f != nil {
		m.err = f
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
