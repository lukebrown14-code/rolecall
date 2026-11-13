package ui_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// openEdit selects an Application by filtering on its title, then opens the Edit form on it.
// Dismissed rows live on the second Leads table.
func (h *addHarness) openEdit(filter string, tabs int) {
	h.t.Helper()
	h.tm.Type("/" + filter)
	h.enter()
	for i := 0; i < tabs; i++ {
		h.tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	h.tm.Type("e")
}

func TestEditFormIsPrefilledFromApplication(t *testing.T) {
	h := newAddHarness(t)
	h.openEdit("Engineer #1", 1)
	h.wait("Title", "Engineer #1", "Example", "Unknown", "https://example.com/status-dismissed", "Good", "Resume only", "Yes")
	h.esc()
	h.quit()
}

func metaOf(t *testing.T, h *addHarness, name string) (string, string) {
	t.Helper()
	p := filepath.Join(h.root, "jobs", "SWE", name, "meta.yml")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return p, strings.TrimSpace(string(b))
}

// saveForm accepts every remaining field of the open Edit form.
func (h *addHarness) saveForm(fields int) {
	h.t.Helper()
	for i := 0; i < fields; i++ {
		h.advance()
	}
}

func TestEditPatchesOnlyChangedKeyAndKeepsConcurrentWrites(t *testing.T) {
	h := newAddHarness(t)
	path, before := metaOf(t, h, "status-dismissed")
	h.openEdit("Engineer #1", 1)
	h.wait("Resume only")
	// An agent writes another key while the form is open. The form never touched it, so it survives.
	if err := os.WriteFile(path, []byte(strings.Replace(before, "fit: Good", "fit: Poor", 1)+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.tm.Type(" Two")
	h.saveForm(7)
	h.wait("Saved · Engineer #1 Two")
	h.quit()

	want := strings.Replace(strings.Replace(before, "fit: Good", "fit: Poor", 1),
		`title: "Engineer #1" # advertised title`, `title: "Engineer #1 Two" # advertised title`, 1)
	if _, got := metaOf(t, h, "status-dismissed"); got != want {
		t.Fatalf("meta.yml\n got: %q\nwant: %q", got, want)
	}
}

func TestEditWithNoChangesWritesNothing(t *testing.T) {
	h := newAddHarness(t)
	before := h.snapshot()
	path, _ := metaOf(t, h, "status-dismissed")
	infoBefore, _ := os.Stat(path)
	h.openEdit("Engineer #1", 1)
	h.wait("Resume only")
	h.saveForm(7)
	h.quit()
	if !sameSnapshot(before, h.snapshot()) {
		t.Fatal("saving an unchanged form wrote to the Workspace")
	}
	if infoAfter, _ := os.Stat(path); !infoAfter.ModTime().Equal(infoBefore.ModTime()) {
		t.Fatal("meta.yml was rewritten")
	}
}

func TestEditTrackUpdatesFieldAndFilterButNotFolder(t *testing.T) {
	h := newAddHarness(t)
	// Fern is a Platform Application whose folder sits under jobs/SWE.
	folder := filepath.Join(h.root, "jobs", "SWE", "fern")
	h.tm.Type("tt") // track filter: All → SWE → Platform
	h.wait("Platform Engineer")
	h.tm.Type("e")
	h.wait("Platform Engineer")
	h.advance()
	h.advance()
	h.tm.Send(tea.KeyPressMsg{Code: tea.KeyUp}) // Platform → SWE
	h.advance()
	h.saveForm(4)
	view := h.quit()

	b, err := os.ReadFile(filepath.Join(folder, "meta.yml"))
	if err != nil {
		t.Fatalf("folder moved: %v", err)
	}
	if !strings.Contains(string(b), "track: \"SWE\"\n") {
		t.Fatalf("track not patched:\n%s", b)
	}
	if !strings.Contains(ansi.Strip(view), "No Leads in Platform") || activeTrack(view) != "Platform" {
		t.Fatalf("Platform filter still shows the row:\n%s", view)
	}
}

func TestEditResumeOnlyTogglesFlagAndDocsMarker(t *testing.T) {
	h := newAddHarness(t)
	h.tm.Type("3")
	for i := 0; i < 3; i++ { // All → Applied → Interviewing → Offer
		h.tm.Send(tea.KeyPressMsg{Code: tea.KeyTab})
	}
	h.tm.Type("/docs-partial")
	h.enter()
	h.wait("docs-partial", "/4")
	h.tm.Type("e")
	h.wait("Resume only", "No")
	h.saveForm(6)
	h.tm.Send(tea.KeyPressMsg{Code: tea.KeyDown}) // No → Yes
	h.advance()
	view := h.quit()

	if _, meta := metaOf(t, h, "docs-partial"); !strings.Contains(meta, "resume_only: \"true\"\n") {
		t.Fatalf("resume_only not set:\n%s", meta)
	}
	if plain := ansi.Strip(view); !strings.Contains(plain, "● PDF") || !strings.Contains(plain, "2/3") {
		t.Fatalf("Docs marker not Ready:\n%s", view)
	}
}

func TestHelpBarOffersEditOnlyOnARow(t *testing.T) {
	onRow := newAddHarness(t).quit()
	h := newAddHarness(t)
	h.tm.Type("/zzznomatch")
	h.enter()
	h.wait("No Applications match")
	empty := h.quit()
	if !strings.Contains(ansi.Strip(onRow), "e edit") || strings.Contains(ansi.Strip(empty), "e edit") {
		t.Fatalf("help bar on a row:\n%s\non an empty table:\n%s", onRow, empty)
	}
}

// activeTrack is the bracketed option of the track filter, which follows its "t" key at
// the end of the context row.
func activeTrack(view string) string {
	for _, line := range strings.Split(ansi.Strip(view), "\n") {
		i := strings.LastIndex(line, " t ")
		if i < 0 {
			continue
		}
		if m := regexp.MustCompile(`\[([^\]]+?)(?: \d+)?\]`).FindStringSubmatch(line[i:]); m != nil {
			return m[1]
		}
	}
	return ""
}
