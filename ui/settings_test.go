package ui_test

import (
	"os"
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
