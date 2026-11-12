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
