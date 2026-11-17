package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui"
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

// unwrapped drops line breaks, spaces and the │ margin glyph, so text the screen wrapped
// (a long path, say) compares equal to the text before it wrapped.
func unwrapped(s string) string {
	return strings.Join(strings.Fields(strings.ReplaceAll(s, "│", " ")), "")
}

// requireUnwrapped is requireAll for text that may wrap: it compares both sides unwrapped.
func requireUnwrapped(t *testing.T, view string, wants ...string) {
	t.Helper()
	got := unwrapped(view)
	for _, want := range wants {
		if !strings.Contains(got, unwrapped(want)) {
			t.Fatalf("missing %q (ignoring wraps):\n%s", want, view)
		}
	}
}

func TestSettingsOpensOnSixAfterProfile(t *testing.T) {
	for _, from := range []string{"1", "4", "5"} {
		view := chipped(runKeys(t, fixtureCopy(t, nil), 120, from, "6"))
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
		"Places", "Australia, APAC", "Remote roles", "count if in my Places")
	requireAll(t, squash(h.tm.plain()), "Blockers 2 read-only")
	h.tm.Type("]")
	requireAll(t, h.tm.plain(), "Max posting age", "Expiry warning", "5 days")
	h.tm.Type("]")
	requireAll(t, h.tm.plain(), "+ add a Source")
	h.tm.Type("]")
	requireAll(t, h.tm.plain(), "Cover letter maximum", "300 words", "Cover letter minimum", "150",
		"Key Skills minimum", "8", "Key Skills maximum", "12")
	h.tm.Type("]")
	requireAll(t, h.tm.plain(), "TRACKS", "SWE", "Follow-up due after", "21 days")
	h.tm.Type("]")
	requireAll(t, h.tm.plain(), "Workspace", h.root[len(h.root)-15:], "Editor", "myeditor", "Downloads", "/dl",
		"Harness", "claude {prompt}", "Plugin", "/opt/plugin")
}

func TestSettingsJKMoveTheFieldCursorWithinASection(t *testing.T) {
	h := newSettingsHarness(t, 120, "", nil)
	h.tm.Type("6")
	requireAll(t, h.tm.plain(), "SETTINGS", "[Search]", "▌ Places")
	h.tm.Type("j")
	v := h.tm.plain()
	requireAll(t, v, "[Search]", "▌ Remote")
	if strings.Contains(v, "▌ Places") {
		t.Fatalf("marker wrong after j:\n%s", v)
	}
}

func TestSettingsEOpensWorkspaceJSONAndCountUpdatesOnReturn(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"sources":{"boardone":[]}}`, func(path string) {
		writeWorkspaceFile(t, filepath.Dir(path), `{"sources":{"boardone":[],"boardtwo":["a"]}}`)
	})
	h.tm.Type("6")
	h.tm.Type("]")
	h.tm.Type("]")
	requireAll(t, squash(h.tm.plain()), "boardone api")
	h.tm.Type("e")
	if got := h.opened(); len(got) != 1 || got[0] != filepath.Join(h.root, "workspace.json") {
		t.Fatalf("editor opened %v", got)
	}
	requireAll(t, squash(h.tm.plain()), "boardtwo api · boardtwo · 1 target · unknown")
}

func TestSettingsReReadsOnEnteringTheScreen(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"search_area":{"places":["Old"]}}`, nil)
	h.tm.Type("6")
	writeWorkspaceFile(t, h.root, `{"search_area":{"places":["Newplace"]}}`)
	h.tm.Type("1")
	h.tm.Type("6")
	requireAll(t, squash(h.tm.plain()), "Places Newplace")
}

func TestSettingsInvalidWorkspaceShowsPathAndErrorAndWritesNothing(t *testing.T) {
	// An invalid file at startup is TestInvalidWorkspaceFileStillStarts; here it breaks
	// while rolecall is running, for example after e.
	bad := `{"tracks":`
	h := newSettingsHarness(t, 120, `{}`, func(string) {})
	h.tm.Type("6")
	writeWorkspaceFile(t, h.root, bad)
	h.tm.Type("1")
	h.tm.Type("6")
	v := h.tm.plain()
	requireUnwrapped(t, v, filepath.Join(h.root, "workspace.json"), "unexpected end of JSON input")
	requireAll(t, v, "e", "edit")
	h.tm.Type("]")
	if strings.Contains(h.tm.plain(), "Cover letter maximum") {
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

// A workspace.json from a newer rolecall is reported, and its settings are neither shown
// nor written: this build would drop keys it does not know.
func TestSettingsNewerSchemaIsReportedAndWritesNothing(t *testing.T) {
	const newer = `{"schema":2,"tracks":["SWE"],"future":{"key":true}}`
	h := newSettingsHarness(t, 120, `{}`, func(string) {})
	h.tm.Type("6")
	writeWorkspaceFile(t, h.root, newer)
	h.tm.Type("1")
	h.tm.Type("6")
	requireUnwrapped(t, h.tm.plain(), "schema 2", filepath.Join(h.root, "workspace.json"), "newer rolecall")
	h.tm.Type("]")
	if strings.Contains(h.tm.plain(), "Cover letter maximum") {
		t.Fatalf("settings shown for a newer-schema file:\n%s", h.tm.plain())
	}
	b, err := os.ReadFile(filepath.Join(h.root, "workspace.json"))
	if err != nil || string(b) != newer {
		t.Fatalf("workspace.json changed: %q, %v", b, err)
	}
}

func TestSettingsNarrowHidesGroupsAndHeaderNamesTheGroup(t *testing.T) {
	h := newSettingsHarness(t, 56, "", nil)
	h.tm.Type("6")
	v := h.tm.plain()
	requireAll(t, v, "[Search] 1 of 6", "Places")
	if strings.Contains(v, "This machine") {
		t.Fatalf("every group named where they do not fit:\n%s", v)
	}
	h.tm.Type("]")
	requireAll(t, h.tm.plain(), "[Rules] 2 of 6", "Max posting age")
}

func TestSettingsHelpListsScreensOneToSix(t *testing.T) {
	h := newSettingsHarness(t, 160, "", nil)
	h.tm.Type("6")
	h.tm.Type("?")
	requireAll(t, helpText(h.tm.plain()), "1-6 screen", "e edit workspace.json")
}
