package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/ui"
)

// settingsHarness drives the Settings screen with a fake editor that records what it was
// given and then rewrites that file, as a real editor would before the TUI resumes.
type settingsHarness struct {
	t    *testing.T
	root string
	tm   *driver
	mu   sync.Mutex
	runs [][]string
	edit func(path string)
}

func newSettingsHarness(t *testing.T, width int, workspaceJSON string, edit func(path string)) *settingsHarness {
	t.Helper()
	root := fixtureCopy(t, func(root string) {
		if workspaceJSON != "" {
			writeWorkspaceFile(t, root, workspaceJSON)
		}
	})
	h := &settingsHarness{t: t, root: root, edit: edit}
	h.tm = newDriver(t, root, fixtureClock, width, 40,
		ui.WithEditor("myeditor"), ui.WithFollowUpDays(21), ui.WithDownloads("/dl"),
		ui.WithHarness("claude {prompt}"), ui.WithPlugin("/opt/plugin"),
		ui.WithExternalRunner(func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
			return func() tea.Msg {
				h.mu.Lock()
				h.runs = append(h.runs, c.Args)
				h.mu.Unlock()
				if h.edit != nil {
					h.edit(c.Args[len(c.Args)-1])
				}
				return done(nil)
			}
		}))
	return h
}

func writeWorkspaceFile(t *testing.T, root, text string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, "workspace.json"), []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

func (h *settingsHarness) opened() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	var files []string
	for _, run := range h.runs {
		files = append(files, run[len(run)-1])
	}
	return files
}

func requireAll(t *testing.T, view string, wants ...string) {
	t.Helper()
	for _, want := range wants {
		if !strings.Contains(view, want) {
			t.Fatalf("missing %q:\n%s", want, view)
		}
	}
}

func TestSettingsOpensOnSixAfterProfile(t *testing.T) {
	for _, from := range []string{"1", "4", "5"} {
		view := ansi.Strip(runKeys(t, fixtureCopy(t, nil), 120, from, "6"))
		requireAll(t, view, "5 Profile", "6 Settings")
		if strings.Index(view, "5 Profile") > strings.Index(view, "6 Settings") {
			t.Fatalf("Settings is not after Profile:\n%s", view)
		}
	}
}

func TestSettingsShowsEveryValueFromItsFile(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"tracks":["SWE"],"search_area":{"places":["Australia","APAC"],"remote":"in-places"},`+
		`"letterWordCap":300,"sources":[1,2,3],"blockers":["a","b"]}`, nil)
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "Search", "Documents", "Tracking", "This machine",
		"Places", "Australia, APAC", "Remote", "in my places", "Sources · 3", "Blockers · 2")
	h.tm.Type("j")
	requireAll(t, h.tm.plain(), "Cover letter word cap", "300", "Cover letter minimum", "150",
		"Key Skills minimum", "8", "Key Skills maximum", "12")
	h.tm.Type("j")
	requireAll(t, h.tm.plain(), "Tracks", "SWE", "Follow-up days", "21")
	h.tm.Type("j")
	requireAll(t, h.tm.plain(), "Workspace", h.root[len(h.root)-40:], "Editor", "myeditor", "Downloads", "/dl",
		"Harness", "claude {prompt}", "Plugin", "/opt/plugin")
}

func TestSettingsTabAndJKMoveRowsInTheSettingsPane(t *testing.T) {
	h := newSettingsHarness(t, 120, "", nil)
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "GROUPS", "▸ Search")
	h.tm.Type("tab")
	requireAll(t, h.tm.plain(), "SETTINGS")
	h.tm.Type("j")
	v := h.tm.plain()
	requireAll(t, v, "Search", "▸ Remote")
	if strings.Contains(v, "▸ Search") {
		t.Fatalf("the unfocused group list kept its marker:\n%s", v)
	}
	if strings.Contains(v, "▸ Places") {
		t.Fatalf("j did not move the row:\n%s", v)
	}
}

func TestSettingsEOpensWorkspaceJSONAndCountUpdatesOnReturn(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"sources":[1,2]}`, func(path string) {
		writeWorkspaceFile(t, filepath.Dir(path), `{"sources":[1,2,3,4,5]}`)
	})
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "Sources · 2")
	h.tm.Type("e")
	if got := h.opened(); len(got) != 1 || got[0] != filepath.Join(h.root, "workspace.json") {
		t.Fatalf("editor opened %v", got)
	}
	requireAll(t, h.tm.plain(), "Sources · 5")
}

func TestSettingsReReadsOnEnteringTheScreen(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"sources":[1]}`, nil)
	h.tm.Type("6")
	writeWorkspaceFile(t, h.root, `{"sources":[1,2,3]}`)
	h.tm.Type("1")
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "Sources · 3")
}

func TestSettingsInvalidWorkspaceShowsPathAndErrorAndWritesNothing(t *testing.T) {
	// An invalid file stops startup (the CLI test in cmd/rolecall); here it breaks while
	// rolecall is running, for example after e.
	bad := `{"tracks":`
	h := newSettingsHarness(t, 120, `{}`, func(string) {})
	h.tm.Type("6")
	writeWorkspaceFile(t, h.root, bad)
	h.tm.Type("1")
	h.tm.Type("6")
	v := h.tm.plain()
	requireAll(t, v, filepath.Join(h.root, "workspace.json"), "unexpected end of JSON input", "e", "edit")
	h.tm.Type("j")
	h.tm.Type("tab")
	if strings.Contains(h.tm.plain(), "Cover letter word cap") {
		t.Fatalf("settings are shown for an invalid file:\n%s", h.tm.plain())
	}
	h.tm.Type("e")
	if len(h.opened()) != 1 {
		t.Fatalf("e did not open the editor: %v", h.opened())
	}
	b, err := os.ReadFile(filepath.Join(h.root, "workspace.json"))
	if err != nil || string(b) != bad {
		t.Fatalf("workspace.json changed: %q, %v", b, err)
	}
}

func TestSettingsNarrowHidesGroupsAndHeaderNamesTheGroup(t *testing.T) {
	h := newSettingsHarness(t, 70, "", nil)
	h.tm.Type("6")
	v := h.tm.plain()
	requireAll(t, v, "Search 1/4", "Places")
	if strings.Contains(v, "This machine") {
		t.Fatalf("group list still shown below 80 columns:\n%s", v)
	}
	h.tm.Type("j")
	requireAll(t, h.tm.plain(), "Documents 2/4", "Cover letter word cap")
}

func TestSettingsHelpListsScreensOneToSix(t *testing.T) {
	h := newSettingsHarness(t, 120, "", nil)
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "1-6", "screen", "e", "edit workspace.json")
}
