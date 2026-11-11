package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// tracksHarness opens Settings on the Tracking group with the settings pane focused.
func tracksHarness(t *testing.T, workspaceJSON string) *settingsHarness {
	t.Helper()
	h := newSettingsHarness(t, 120, workspaceJSON, nil)
	h.tm.Type("6")
	h.tm.Type("j")
	h.tm.Type("j")
	h.tm.Type("tab")
	return h
}

func (h *settingsHarness) file() string {
	h.t.Helper()
	b, err := os.ReadFile(filepath.Join(h.root, "workspace.json"))
	if err != nil {
		h.t.Fatal(err)
	}
	return string(b)
}

func TestSettingsAddTrackWritesFileAndHeaderFollows(t *testing.T) {
	h := tracksHarness(t, `{"sources":[1],"tracks":["SWE"]}`)
	h.tm.Type("a")
	h.tm.Type("Data")
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "SWE", "Data")
	f := h.file()
	if !strings.Contains(f, `"Data"`) || strings.Index(f, `"sources"`) > strings.Index(f, `"tracks"`) {
		t.Fatalf("file = %s", f)
	}
	h.tm.Type("1")
	requireAll(t, h.tm.plain(), "Data") // header filter lists the new Track
}

func TestSettingsAddTrackRules(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE"]}`)
	for name, want := range map[string]string{
		"   ": "empty",
		"swe": "already exists",
		"A/B": "/",
	} {
		h.tm.Type("a")
		h.tm.Type(name)
		h.tm.Type("enter")
		requireAll(t, h.tm.plain(), want)
		h.tm.Type("esc")
	}
	if strings.Contains(h.file(), "A/B") {
		t.Fatalf("invalid name written: %s", h.file())
	}
}

func TestSettingsRemoveTrackRefusedWhileUsedWithCount(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Spare"]}`)
	h.tm.Type("x")
	requireAll(t, h.tm.plain(), "SWE is used by 10 Applications")
	requireAll(t, h.file(), `"SWE"`)
}

func TestSettingsRemoveUnusedAndLastTrack(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["Spare"]}`)
	h.tm.Type("x")
	if strings.Contains(h.file(), "Spare") {
		t.Fatalf("not removed: %s", h.file())
	}
	requireAll(t, h.tm.plain(), "none")
	h.tm.Type("1")
	if strings.Contains(h.tm.plain(), "Spare") {
		t.Fatalf("header filter kept removed Track:\n%s", h.tm.plain())
	}
}

func TestSettingsReorderTracks(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["A","B","C"]}`)
	h.tm.Type("J")
	f := h.file()
	if !(strings.Index(f, `"B"`) < strings.Index(f, `"A"`) && strings.Index(f, `"A"`) < strings.Index(f, `"C"`)) {
		t.Fatalf("J did not move A down: %s", f)
	}
	h.tm.Type("K")
	f = h.file()
	if !(strings.Index(f, `"A"`) < strings.Index(f, `"B"`)) {
		t.Fatalf("K did not move it back: %s", f)
	}
}

func TestSettingsTrackKeysNeverWriteInvalidJSON(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"tracks": [`, nil)
	h.tm.Type("6")
	h.tm.Type("a")
	if h.file() != `{"tracks": [` {
		t.Fatalf("file changed: %s", h.file())
	}
}
