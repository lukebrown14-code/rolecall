package ui_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// squash collapses column padding and dividers so a row reads as "Label value (source)".
func squash(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "│", " ")), " ")
}

// sourcesHarness is the Settings screen over a config.toml in a temporary home, loaded the
// way main loads it. edit, when set, rewrites the config while the fake Editor "runs".
type sourcesHarness struct {
	cfg  string
	cmds [][]string
	d    *driver
}

func newSourcesHarness(t *testing.T, width int, initial string, edit func(path string), extra ...ui.Option) *sourcesHarness {
	t.Helper()
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	t.Setenv("ROLECALL_PLUGIN", "")
	home := t.TempDir()
	h := &sourcesHarness{cfg: config.Path(home)}
	if initial != "" {
		if err := os.MkdirAll(filepath.Dir(h.cfg), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(h.cfg, []byte(initial), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	root := fixtureCopy(t, nil)
	c, err := config.Load(home, root)
	if err != nil {
		t.Fatal(err)
	}
	runner := func(cmd *exec.Cmd, cb tea.ExecCallback) tea.Cmd {
		h.cmds = append(h.cmds, cmd.Args)
		return func() tea.Msg {
			if edit != nil {
				edit(h.cfg)
			}
			return cb(nil)
		}
	}
	opts := append([]ui.Option{ui.WithConfigHome(home), ui.WithExternalRunner(runner), ui.WithEditor(c.Editor),
		ui.WithEditorSource(c.EditorSource), ui.WithFollowUpDays(c.FollowUpDays), ui.WithDownloads(c.Downloads),
		ui.WithPlugin(c.Plugin), ui.WithHarness(c.Harness),
		ui.WithSettingSources(ui.SettingSources{Workspace: c.WorkspaceSource, FollowUpDays: c.FollowUpDaysSource,
			Downloads: c.DownloadsSource, Plugin: c.PluginSource, Harness: c.HarnessSource})}, extra...)
	h.d = newDriver(t, root, fixtureClock, width, 40, opts...)
	return h
}

// machine opens Settings on its This machine group.
func (h *sourcesHarness) machine() string {
	h.d.Type("6")
	for range 3 {
		h.d.Type("]")
	}
	return squash(h.d.plain())
}

func TestSettingsNamesEachMachineSettingsSource(t *testing.T) {
	for _, tc := range []struct {
		name, toml, visual, editor, plugin string
		want                               []string
	}{
		{"config", "editor = \"code --wait\"\ndownloads = \"/srv/dl\"\nharness = \"claude {skill}\"\nplugin = \"/opt/p\"\n", "vim", "nano", "",
			[]string{"Editor code --wait config.toml", "Downloads /srv/dl config.toml", "Harness claude {skill} config.toml", "Plugin /opt/p config.toml"}},
		{"visual", "", "vim", "nano", "", []string{"Editor vim from $VISUAL"}},
		{"editor", "", "", "nano", "/env/p", []string{"Editor nano from $EDITOR", "Plugin /env/p from $ROLECALL_PLUGIN"}},
		{"default", "", "", "", "", []string{"Editor vi default", "Downloads", "default", "Plugin not set"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newSourcesHarness(t, 140, tc.toml, nil)
			t.Setenv("VISUAL", tc.visual)
			t.Setenv("EDITOR", tc.editor)
			t.Setenv("ROLECALL_PLUGIN", tc.plugin)
			// Re-enter so config.toml and the environment are read afresh.
			h.d.Type("1")
			v := h.machine()
			for _, w := range tc.want {
				if !strings.Contains(v, w) {
					t.Fatalf("Settings lacks %q:\n%s", w, h.d.plain())
				}
			}
		})
	}
}

func TestSettingsNamesWorkspaceAndFollowUpSources(t *testing.T) {
	h := newSourcesHarness(t, 140, "follow_up_days = 7\n", nil)
	h.d.Type("6")
	h.d.Type("]")
	h.d.Type("]")
	if v := squash(h.d.plain()); !strings.Contains(v, "Follow-up due after 7 days config.toml") {
		t.Fatalf("Follow-up source missing:\n%s", h.d.plain())
	}
	h.d.Type("]")
	if v := squash(h.d.plain()); !strings.Contains(v, "from --dir") {
		t.Fatalf("Workspace source missing:\n%s", h.d.plain())
	}
}

func TestSettingsExplicitZeroFollowUpDaysIsKept(t *testing.T) {
	h := newSourcesHarness(t, 140, "follow_up_days = 0\n", nil)
	h.d.Type("6")
	h.d.Type("]")
	h.d.Type("]")
	if v := squash(h.d.plain()); !strings.Contains(v, "Follow-up due after 0 days config.toml") {
		t.Fatalf("explicit 0 lost:\n%s", h.d.plain())
	}
}

func TestSettingsWarnsWhenEditorNotOnPath(t *testing.T) {
	dir := t.TempDir()
	short, err := os.MkdirTemp("/tmp", "r")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(short) })
	spaced := filepath.Join(short, "a b")
	if err := os.MkdirAll(spaced, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{dir, spaced} {
		if err := os.WriteFile(filepath.Join(d, "fe"), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	exe := filepath.Join(spaced, "fe")
	t.Setenv("PATH", dir)
	const warn = "⚠ not found on PATH"
	for _, tc := range []struct {
		name, editor string
		warn         bool
	}{
		{"missing", "nosuch", true},
		{"found", "fe", false},
		{"found with flags", "fe -w -x", false},
		{"missing with flags", "nosuch -w", true},
		{"quoted path found", `"` + exe + `" -w`, false},
		{"single quoted path found", `'` + exe + `'`, false},
		{"quoted path missing", `"/no such/x"`, true},
		{"flag names a missing program", "fe nosuch", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newSourcesHarness(t, 200, fmt.Sprintf("editor = %q\n", tc.editor), nil)
			if got := strings.Contains(h.machine(), warn); got != tc.warn {
				t.Fatalf("warning shown = %v, want %v:\n%s", got, tc.warn, h.d.plain())
			}
		})
	}
}

func TestSettingsNarrowKeepsSourceAndWarningWhole(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	h := newSourcesHarness(t, 60, "editor = \"nosuch-editor-binary --wait --new-window --some-very-long-flag\"\n", nil)
	if v := h.machine(); !strings.Contains(v, "config.toml ⚠ not found on PATH") {
		t.Fatalf("narrow Settings cut the source or warning:\n%s", h.d.plain())
	}
}

func TestSettingsEditOnMachineOpensConfigAndCreatesIt(t *testing.T) {
	h := newSourcesHarness(t, 120, "", nil)
	h.machine()
	h.d.Type("e")
	if _, err := os.Stat(h.cfg); err != nil {
		t.Fatalf("config.toml not created: %v", err)
	}
	if len(h.cmds) != 1 || h.cmds[0][len(h.cmds[0])-1] != h.cfg || !strings.Contains(strings.Join(h.cmds[0], " "), "vi") {
		t.Fatalf("Editor not launched on config.toml: %q", h.cmds)
	}
}

func TestSettingsEditOutsideMachineStillOpensWorkspaceJSON(t *testing.T) {
	h := newSourcesHarness(t, 120, "", nil)
	h.d.Type("6")
	h.d.Type("e")
	if len(h.cmds) != 1 || !strings.HasSuffix(h.cmds[0][len(h.cmds[0])-1], "workspace.json") {
		t.Fatalf("e on Search should open workspace.json: %q", h.cmds)
	}
}

func TestSettingsEditReloadsConfigOnExit(t *testing.T) {
	h := newSourcesHarness(t, 140, "editor = \"nvim\"\n", func(p string) {
		_ = os.WriteFile(p, []byte("editor = \"hx\"\nfollow_up_days = 3\ndownloads = \"/srv/dl\"\n"), 0o600)
	})
	h.machine()
	h.d.Type("e")
	v := squash(h.d.plain())
	for _, w := range []string{"Editor hx config.toml", "Downloads /srv/dl config.toml", "from --dir"} {
		if !strings.Contains(v, w) {
			t.Fatalf("Settings not refreshed, lacks %q:\n%s", w, h.d.plain())
		}
	}
}

func TestSettingsEditKeepsOldValuesAndShowsErrorOnInvalidTOML(t *testing.T) {
	// Neither Editor exists: its independent PATH warning must survive a good reload.
	t.Setenv("PATH", t.TempDir())
	statusLine := func(view string) string {
		// The feedback line sits above the key line, so it is the second-to-last line.
		lines := strings.Split(strings.TrimSpace(view), "\n")
		return lines[len(lines)-2]
	}
	n := 0
	h := newSourcesHarness(t, 140, "editor = \"nvim\"\n", func(p string) {
		n++
		if n == 1 {
			_ = os.WriteFile(p, []byte("editor = = \n"), 0o600)
		} else {
			_ = os.WriteFile(p, []byte("editor = \"hx\"\n"), 0o600)
		}
	})
	h.machine()
	h.d.Type("e")
	if v := h.d.plain(); !strings.Contains(squash(v), "Editor nvim config.toml") || !strings.Contains(statusLine(v), "✘") {
		t.Fatalf("old values and error not shown:\n%s", h.d.plain())
	}
	h.d.Type("e") // fixing it clears the error
	if v := h.d.plain(); strings.Contains(statusLine(v), "✘") || !strings.Contains(squash(v), "Editor hx config.toml") || !strings.Contains(v, "⚠ not found on PATH") {
		t.Fatalf("error not cleared by a good reload:\n%s", v)
	}
}

func TestSettingsListsEditorExamples(t *testing.T) {
	h := newSourcesHarness(t, 140, "", nil)
	h.machine()
	v := squash(h.d.plain())
	for _, w := range []string{"nvim", "code --wait", "zed --wait", "hx", "subl --wait"} {
		if !strings.Contains(v, w) {
			t.Fatalf("Settings lacks Editor example %q:\n%s", w, h.d.plain())
		}
	}
}

func TestCommaOpensSettingsFromAnywhereAndEscReturns(t *testing.T) {
	for _, keys := range [][]string{{"1"}, {"2"}, {"3"}, {"4"}, {"5"}, {"1", "j"}, {"1", "enter"}, {"/pl", "enter", "j"}} {
		h := newSourcesHarness(t, 140, "", nil)
		h.d.Type("2")
		for _, k := range keys {
			h.d.Type(k)
		}
		before := h.d.plain()
		h.d.Type(",")
		if v := h.d.plain(); !strings.Contains(v, "SETTINGS") && !strings.Contains(v, "GROUPS") {
			t.Fatalf("keys %q: comma did not open Settings:\n%s", keys, v)
		}
		h.d.Type("esc")
		if after := h.d.plain(); after != before {
			t.Fatalf("keys %q: Esc did not restore the view\nbefore:\n%s\nafter:\n%s", keys, before, after)
		}
	}
}

func TestEscInSettingsOpenedByDigitDoesNotLeave(t *testing.T) {
	h := newSourcesHarness(t, 140, "", nil)
	h.d.Type("1")
	h.d.Type("6")
	before := h.d.plain()
	h.d.Type("esc")
	if h.d.plain() != before {
		t.Fatal("Esc left a Settings screen that was not opened with a comma")
	}
}

func TestCommaTypesIntoFilterInsteadOfOpeningSettings(t *testing.T) {
	h := newSourcesHarness(t, 140, "", nil)
	h.d.Type("1")
	h.d.Type("/")
	h.d.Type(",")
	if v := h.d.plain(); strings.Contains(v, "GROUPS") {
		t.Fatalf("comma in the filter opened Settings:\n%s", v)
	}
}

func TestCommaIsInFullHelp(t *testing.T) {
	for _, screen := range []string{"1", "4", "5"} {
		h := newSourcesHarness(t, 140, "", nil)
		h.d.Type(screen)
		h.d.Type("?")
		if v := h.d.plain(); !strings.Contains(v, ",") || !strings.Contains(v, "settings") {
			t.Fatalf("screen %s: help does not list , settings:\n%s", screen, v)
		}
	}
}
