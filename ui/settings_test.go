package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/ui"
)

// settingsOpts loads the Owner's config from a temporary home, as main does, and returns
// the options that carry its Editor and where it came from.
func settingsOpts(t *testing.T, root, configTOML string) []ui.Option {
	t.Helper()
	home := t.TempDir()
	if configTOML != "" {
		p := config.Path(home)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(configTOML), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	c, err := config.Load(home, root)
	if err != nil {
		t.Fatal(err)
	}
	return []ui.Option{ui.WithEditor(c.Editor), ui.WithEditorSource(c.EditorSource)}
}

func TestSettingsOpensFromEveryScreenAndEscRestores(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"1", "enter"}, {"/pl", "enter", "j"}} {
		m := driveStart(t, root, fixtureClock, 120, 40, settingsOpts(t, root, "editor = \"nvim\"\n")...)
		m = driveModel(t, m, keys...)
		before := plainView(m)
		m = driveModel(t, m, ",")
		open := plainView(m)
		if !strings.Contains(open, "Settings") || !strings.Contains(open, "Editor: nvim (config.toml)") {
			t.Fatalf("keys %q: Settings not shown:\n%s", keys, open)
		}
		m = driveModel(t, m, "esc")
		if after := plainView(m); after != before {
			t.Fatalf("keys %q: Esc did not restore the view\nbefore:\n%s\nafter:\n%s", keys, before, after)
		}
	}
}

func plainView(m tea.Model) string { return ansi.Strip(m.View().Content) }

func TestSettingsNamesTheEditorSource(t *testing.T) {
	for _, tc := range []struct {
		name, toml, visual, editor, want string
	}{
		{"config wins", "editor = \"code --wait\"\n", "vim", "nano", "Editor: code --wait (config.toml)"},
		{"visual", "", "vim", "nano", "Editor: vim ($VISUAL)"},
		{"editor", "", "", "nano", "Editor: nano ($EDITOR)"},
		{"default", "", "", "", "Editor: vi (default)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("VISUAL", tc.visual)
			t.Setenv("EDITOR", tc.editor)
			root := fixtureCopy(t, nil)
			m := driveStart(t, root, fixtureClock, 120, 40, settingsOpts(t, root, tc.toml)...)
			if v := plainView(driveModel(t, m, ",")); !strings.Contains(v, tc.want) {
				t.Fatalf("Settings lacks %q:\n%s", tc.want, v)
			}
		})
	}
}

func TestSettingsKeyIsInFullHelp(t *testing.T) {
	root := fixtureCopy(t, nil)
	for _, keys := range [][]string{{"?"}, {"4", "?"}, {"1", "enter", "?"}} {
		m := driveStart(t, root, fixtureClock, 120, 40)
		if v := plainView(driveModel(t, m, keys...)); !strings.Contains(v, ", ") || !strings.Contains(v, "settings") {
			t.Fatalf("keys %q: help does not list , settings:\n%s", keys, v)
		}
	}
}

// editorHarness is Settings with a config file in a temporary home and a fake Editor
// that records its command and, when set, rewrites the config as the Owner would.
type editorHarness struct {
	cfgPath string
	cmds    [][]string
	m       tea.Model
}

func newEditorHarness(t *testing.T, initial string, edit func(path string)) *editorHarness {
	t.Helper()
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	root := fixtureCopy(t, nil)
	home := t.TempDir()
	h := &editorHarness{cfgPath: config.Path(home)}
	if initial != "" {
		if err := os.MkdirAll(filepath.Dir(h.cfgPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(h.cfgPath, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	runner := func(c *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		h.cmds = append(h.cmds, c.Args)
		return func() tea.Msg {
			if edit != nil {
				edit(h.cfgPath)
			}
			return cb(nil)
		}
	}
	c, err := config.Load(home, root)
	if err != nil {
		t.Fatal(err)
	}
	h.m = driveStart(t, root, fixtureClock, 120, 40, ui.WithExternalRunner(runner),
		ui.WithEditor(c.Editor), ui.WithEditorSource(c.EditorSource),
		ui.WithConfigReload(h.cfgPath, func() (config.Config, error) { return config.Load(home, root) }))
	return h
}

func TestSettingsEditOpensConfigInEditorAndCreatesMissingFile(t *testing.T) {
	h := newEditorHarness(t, "", nil)
	h.m = driveModel(t, h.m, ",", "e")
	if _, err := os.Stat(h.cfgPath); err != nil {
		t.Fatalf("config file not created: %v", err)
	}
	if len(h.cmds) != 1 || h.cmds[0][len(h.cmds[0])-1] != h.cfgPath || !strings.Contains(strings.Join(h.cmds[0], " "), "vi") {
		t.Fatalf("Editor not launched on the config file: %q", h.cmds)
	}
}

func TestSettingsEditAppliesValidChangeWithoutRestart(t *testing.T) {
	h := newEditorHarness(t, "editor = \"nvim\"\n", func(p string) { _ = os.WriteFile(p, []byte("editor = \"hx\"\n"), 0o600) })
	h.m = driveModel(t, h.m, ",", "e")
	if v := plainView(h.m); !strings.Contains(v, "Editor: hx (config.toml)") {
		t.Fatalf("Editor line not updated:\n%s", v)
	}
}

func TestSettingsEditKeepsOldValuesOnInvalidTOML(t *testing.T) {
	h := newEditorHarness(t, "editor = \"nvim\"\n", func(p string) { _ = os.WriteFile(p, []byte("editor = = \n"), 0o600) })
	h.m = driveModel(t, h.m, ",", "e")
	v := plainView(h.m)
	if !strings.Contains(v, "Editor: nvim (config.toml)") || !strings.Contains(v, "✕") {
		t.Fatalf("old values and error not shown:\n%s", v)
	}
}

func TestSettingsHelpLineListsEditorExamples(t *testing.T) {
	h := newEditorHarness(t, "", nil)
	v := plainView(driveModel(t, h.m, ","))
	for _, want := range []string{"e edit config", "nvim", "code --wait", "zed --wait", "hx", "subl --wait"} {
		if !strings.Contains(v, want) {
			t.Fatalf("Settings lacks %q:\n%s", want, v)
		}
	}
}
