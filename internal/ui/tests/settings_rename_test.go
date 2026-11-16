package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func (h *settingsHarness) metaTrack(rel string) string {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.root, "jobs", rel, "meta.yml"))
	if err != nil {
		h.t.Fatal(err)
	}
	for _, l := range strings.Split(string(b), "\n") {
		if v, ok := strings.CutPrefix(l, "track:"); ok {
			return strings.Trim(strings.TrimSpace(v), `"`)
		}
	}
	return ""
}

// startRename presses r on SWE and types the new name, stopping at the confirm pop-up.
func (h *settingsHarness) startRename(name string) {
	h.tm.Type("r")
	for range "SWE" {
		h.tm.Type("backspace")
	}
	h.tm.Type(name)
	h.tm.Type("enter")
}

func TestSettingsRenameConfirmShowsCountsAndUndoWarning(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	h.startRename("Engineering")
	requireAll(t, h.tm.plain(), "Rename Track", "SWE", "Engineering", "10 Applications", "roles.json", "undo")
	if got := h.metaTrack("SWE/orbit"); got != "SWE" {
		t.Fatalf("written before confirm: %q", got)
	}
}

func TestSettingsRenameCancelChangesNothing(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	h.startRename("Engineering")
	h.tm.Type("esc")
	if got := h.metaTrack("SWE/orbit"); got != "SWE" || strings.Contains(h.file(), "Engineering") {
		t.Fatalf("cancel wrote: %q %s", got, h.file())
	}
}

func TestSettingsRenameConfirmRewritesAndReportsMalformed(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	h.startRename("Engineering")
	h.tm.Type("y")
	if got := h.metaTrack("SWE/orbit"); got != "Engineering" {
		t.Fatalf("orbit track = %q", got)
	}
	if !strings.Contains(h.file(), `"Engineering"`) || strings.Contains(h.file(), `"SWE"`) {
		t.Fatalf("workspace.json = %s", h.file())
	}
	roles, _ := os.ReadFile(filepath.Join(h.root, "jobs", "roles.json"))
	if strings.Contains(string(roles), `"SWE"`) {
		t.Fatalf("roles.json = %s", roles)
	}
	if _, err := os.Stat(filepath.Join(h.root, "jobs", "SWE", "orbit", "meta.yml")); err != nil {
		t.Fatalf("folder moved: %v", err)
	}
	requireAll(t, h.tm.plain(), "malformed") // named after the rename
	requireAll(t, h.tm.plain(), "SWE/malformed")
	h.tm.Type("esc")
	h.tm.Type("1")
	requireAll(t, h.tm.plain(), "Engineering") // header filter follows
}

func TestSettingsRenameAppliesNameRules(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	for name, want := range map[string]string{"platform": "already exists", "A/B": "/", "": "required"} {
		h.tm.Type("r")
		for range "SWE" {
			h.tm.Type("backspace")
		}
		h.tm.Type(name)
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), want)
		h.tm.Type("esc")
		h.tm.Type("y") // discard the typed name
	}
	if got := h.metaTrack("SWE/orbit"); got != "SWE" {
		t.Fatalf("invalid rename wrote: %q", got)
	}
}

func TestSettingsRenameIsNotUndoneByU(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	h.startRename("Engineering")
	h.tm.Type("y")
	h.tm.Type("esc")
	h.tm.Type("u")
	if got := h.metaTrack("SWE/orbit"); got != "Engineering" {
		t.Fatalf("u undid the rename: %q", got)
	}
}
