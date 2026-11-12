package ui_test

import (
	"fmt"
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
	return []ui.Option{ui.WithEditor(c.Editor), ui.WithEditorSource(c.EditorSource),
		ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads), ui.WithPlugin(c.Plugin), ui.WithHarness(c.Harness),
		ui.WithSettingSources(ui.SettingSources{Workspace: c.WorkspaceSource, FollowUpDays: c.FollowUpDaysSource,
			Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource})}
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

// fakeEditorDir returns a PATH directory holding an executable named name, and a
// second directory whose name contains a space, holding the same.
func fakeEditorDir(t *testing.T, name string) (pathDir, spacedExe string) {
	t.Helper()
	pathDir = t.TempDir()
	// A short base keeps the whole Editor line inside the popup.
	short, err := os.MkdirTemp("/tmp", "r")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })
	spaced := filepath.Join(short, "a b")
	if err := os.MkdirAll(spaced, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{pathDir, spaced} {
		if err := os.WriteFile(filepath.Join(d, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return pathDir, filepath.Join(spaced, name)
}

func TestSettingsWarnsWhenEditorNotOnPath(t *testing.T) {
	pathDir, spacedExe := fakeEditorDir(t, "fe")
	t.Setenv("PATH", pathDir)
	const warn = "(not found on PATH)"
	for _, tc := range []struct {
		name, editor string
		warn         bool
	}{
		{"missing", "nosuch", true},
		{"found", "fe", false},
		{"found with flags", "fe -w -x", false},
		{"missing with flags", "nosuch -w", true},
		{"quoted path found", `"` + spacedExe + `" -w`, false},
		{"single quoted path found", `'` + spacedExe + `'`, false},
		{"quoted path missing", `"/no such/x"`, true},
		{"flag names a missing program", "fe nosuch", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := fixtureCopy(t, nil)
			toml := fmt.Sprintf("editor = %q\n", tc.editor)
			m := driveStart(t, root, fixtureClock, 200, 40, settingsOpts(t, root, toml)...)
			v := plainView(driveModel(t, m, ","))
			if got := strings.Contains(v, warn); got != tc.warn {
				t.Fatalf("warning shown = %v, want %v:\n%s", got, tc.warn, v)
			}
			// Never blocks: Settings still opens and closes.
			if v := plainView(driveModel(t, m, ",", "esc")); strings.Contains(v, "Settings") {
				t.Fatalf("Settings did not close:\n%s", v)
			}
		})
	}
}

func TestSettingsListsEverySettingWithSourceEditorFirst(t *testing.T) {
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("ROLECALL_PLUGIN", "")
	root := fixtureCopy(t, nil)
	toml := "editor = \"nvim\"\nfollow_up_days = 7\ndownloads = \"/srv/dl\"\nplugin = \"/opt/plugin\"\nharness = \"claude {skill} {dir}\"\n"
	m := driveStart(t, root, fixtureClock, 120, 40, settingsOpts(t, root, toml)...)
	v := plainView(driveModel(t, m, ","))
	want := []string{"Editor: nvim (config.toml)", "Follow-up days: 7 (config.toml)",
		"Downloads: /srv/dl (config.toml)", "Plugin: /opt/plugin (config.toml)", "Harness: claude {skill} {dir} (config.toml)"}
	if flat := strings.NewReplacer("│", "", " ", "", "\n", "").Replace(v); !strings.Contains(flat, "Workspace:"+root+"(--dir)") {
		t.Fatalf("Settings lacks the Workspace and its source:\n%s", v)
	}
	last := -1
	for _, w := range want {
		i := strings.Index(v, w)
		if i < 0 {
			t.Fatalf("Settings lacks %q:\n%s", w, v)
		}
		if i < last {
			t.Fatalf("%q out of order:\n%s", w, v)
		}
		last = i
	}
}

func TestSettingsShowsDefaultsForUnsetSettings(t *testing.T) {
	t.Setenv("ROLECALL_PLUGIN", "")
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40, settingsOpts(t, root, "")...)
	v := plainView(driveModel(t, m, ","))
	// The default Downloads folder sits under the Owner's home, which wraps in a temp one.
	if flat := strings.NewReplacer("│", "", " ", "", "\n", "").Replace(v); !strings.Contains(flat, "/Downloads(default)") {
		t.Fatalf("Downloads default not shown:\n%s", v)
	}
	for _, w := range []string{"Follow-up days: 14 (default)", "Plugin: not set (default)", "Harness: not set (default)"} {
		if !strings.Contains(v, w) {
			t.Fatalf("Settings lacks %q:\n%s", w, v)
		}
	}
}

func TestSettingsNamesPluginEnvironmentSource(t *testing.T) {
	t.Setenv("ROLECALL_PLUGIN", "/env/plugin")
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40, settingsOpts(t, root, "")...)
	if v := plainView(driveModel(t, m, ",")); !strings.Contains(v, "Plugin: /env/plugin ($ROLECALL_PLUGIN)") {
		t.Fatalf("Settings lacks plugin env source:\n%s", v)
	}
}

func TestSettingsStaysReadableOnNarrowTerminal(t *testing.T) {
	t.Setenv("ROLECALL_PLUGIN", "")
	root := fixtureCopy(t, nil)
	long := "/very/long/path/to/the/downloads/directory/that/will/not/fit"
	m := driveStart(t, root, fixtureClock, 50, 40, settingsOpts(t, root, "downloads = \""+long+"\"\n")...)
	v := plainView(driveModel(t, m, ","))
	if strings.Contains(v, "…") {
		t.Fatalf("narrow Settings truncates values:\n%s", v)
	}
	flat := strings.NewReplacer("│", "", " ", "", "\n", "").Replace(v)
	if !strings.Contains(flat, strings.ReplaceAll(long, " ", "")+"(config.toml)") {
		t.Fatalf("narrow Settings lost the value or its source:\n%s", v)
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
		ui.WithSettingSources(ui.SettingSources{Workspace: c.WorkspaceSource, FollowUpDays: c.FollowUpDaysSource,
			Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource}),
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

func TestSettingsEditRefreshesEverySettingAndItsSource(t *testing.T) {
	t.Setenv("ROLECALL_PLUGIN", "")
	h := newEditorHarness(t, "", func(p string) {
		_ = os.WriteFile(p, []byte("follow_up_days = 3\ndownloads = \"/srv/dl\"\nplugin = \"/opt/plugin\"\nharness = \"claude {skill}\"\n"), 0o600)
	})
	h.m = driveModel(t, h.m, ",", "e")
	v := plainView(h.m)
	for _, w := range []string{"Follow-up days: 3 (config.toml)", "Downloads: /srv/dl (config.toml)", "Plugin: /opt/plugin (config.toml)", "Harness: claude {skill} (config.toml)"} {
		if !strings.Contains(v, w) {
			t.Fatalf("Settings not refreshed, lacks %q:\n%s", w, v)
		}
	}
	if flat := strings.NewReplacer("│", "", " ", "", "\n", "").Replace(v); !strings.Contains(flat, "(--dir)") {
		t.Fatalf("Workspace source lost after reload:\n%s", v)
	}
}

func TestSettingsNarrowKeepsMissingEditorWarningUnbroken(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := fixtureCopy(t, nil)
	toml := "editor = \"nosuch-editor-binary --wait --new-window --some-very-long-flag\"\n"
	m := driveStart(t, root, fixtureClock, 50, 40, settingsOpts(t, root, toml)...)
	v := plainView(driveModel(t, m, ","))
	found := false
	for _, l := range strings.Split(v, "\n") {
		if strings.Contains(l, "(not found on PATH)") {
			found = true
		}
	}
	if !found {
		t.Fatalf("warning missing or broken mid-phrase:\n%s", v)
	}
}

func TestCommaTypesIntoFilterInsteadOfOpeningSettings(t *testing.T) {
	root := fixtureCopy(t, nil)
	m := driveStart(t, root, fixtureClock, 120, 40, settingsOpts(t, root, "")...)
	m = driveModel(t, m, "/", ",")
	if v := plainView(m); strings.Contains(v, "Settings") {
		t.Fatalf("comma in the filter opened Settings:\n%s", v)
	}
}

func TestSettingsReloadClearsEarlierError(t *testing.T) {
	n := 0
	h := newEditorHarness(t, "editor = \"nvim\"\n", func(p string) {
		n++
		if n == 1 {
			_ = os.WriteFile(p, []byte("editor = = \n"), 0o600)
		} else {
			_ = os.WriteFile(p, []byte("editor = \"hx\"\n"), 0o600)
		}
	})
	h.m = driveModel(t, h.m, ",", "e")
	if !strings.Contains(plainView(h.m), "✕") {
		t.Fatal("expected error after invalid edit")
	}
	h.m = driveModel(t, h.m, "e")
	if v := plainView(h.m); strings.Contains(v, "✕") {
		t.Fatalf("error not cleared after successful reload:\n%s", v)
	}
}
