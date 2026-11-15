package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A Harness not on PATH and a Plugin folder that is missing, or lacks skills/, are saved
// but warned about, as an Editor not on PATH is.
func TestSettingsHarnessAndPluginWarnButSave(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 3, 2)
	d.Type("enter")
	d.Type("no-such-harness-xyz {dir}")
	d.Type("enter")
	d.Type("j")
	d.Type("enter")
	d.Type("/no/such/plugin")
	d.Type("enter")
	b, _ := os.ReadFile(cfg)
	if !strings.Contains(string(b), `harness = "no-such-harness-xyz {dir}"`) || !strings.Contains(string(b), `plugin = "/no/such/plugin"`) {
		t.Fatalf("not saved:\n%s", b)
	}
	v := d.plain()
	requireAll(t, v, "no-such-harness-xyz not found on PATH", "folder not found")

	plugin := t.TempDir()
	d.Type("enter")
	d.Type(plugin)
	d.Type("enter")
	requireAll(t, d.plain(), "no skills/ folder in it")
	if err := os.Mkdir(filepath.Join(plugin, "skills"), 0o700); err != nil {
		t.Fatal(err)
	}
	d.Type("r")
	if strings.Contains(d.plain(), "no skills/ folder") || strings.Contains(d.plain(), "folder not found") {
		t.Fatalf("warned about a good Plugin:\n%s", d.plain())
	}
}

// The focused row says what it is: beside the rows when wide, under them otherwise. The
// Harness example stays in view while the value is edited.
func TestSettingsExplainsTheFocusedRow(t *testing.T) {
	for _, w := range []int{120, 80} {
		lines := settingsAt(t, w, 30, "]", "]", "]", "j", "j")
		v := strings.Join(lines, "\n")
		requireAll(t, squash(v), "AI coding tool")
		lines = settingsAt(t, w, 30, "]", "]", "]", "j", "j", "j")
		requireAll(t, squash(strings.Join(lines, "\n")), "skills/ folder")
	}
	// Sources and Blockers are read-only, so the cursor never rests on them: each says what
	// it is in its row, and the group's own text says where they are set.
	search := squash(strings.Join(settingsAt(t, 120, 30, "k"), "\n"))
	requireAll(t, search, "read-only · job boards the Sweep reads", "read-only · signals that rule a role out",
		"Sources and Blockers are set in workspace.json")
	editing := strings.Join(settingsAt(t, 120, 30, "]", "]", "]", "j", "j", "enter"), "\n")
	requireAll(t, editing, `▏claude "/{skill} {dir}"`) // the example as the empty field's placeholder
	typed := strings.Join(settingsAt(t, 120, 30, "]", "]", "]", "j", "j", "enter", "x"), "\n")
	requireAll(t, typed, `example: claude "/{skill} {dir}"`) // and beside it once typing hides the placeholder
}

// A Track Applications use but workspace.json does not list, such as Unknown on old
// Applications, has a row; Enter adds it, and then it renames like any other.
func TestSettingsOffListTrackCanBeAddedAndRenamed(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	requireAll(t, squash(h.tm.plain()), "Unknown 6 applications · not listed")
	for range 2 {
		h.tm.Type("j")
	}
	requireAll(t, h.tm.plain(), "▌ Unknown")
	requireAll(t, keyLineOf(h.tm.View()), "enter add to Tracks")
	h.tm.Type("enter")
	requireAll(t, h.file(), `"Unknown"`)
	requireAll(t, h.tm.plain(), "Added Track Unknown · workspace.json", "▌ Unknown")
	if strings.Contains(h.tm.plain(), "not listed") {
		t.Fatalf("still off the list:\n%s", h.tm.plain())
	}
	h.tm.Type("r")
	requireAll(t, h.tm.plain(), "Rename Track Unknown")
	h.tm.Type("esc")
	h.tm.Type("u")
	if strings.Contains(h.file(), `"Unknown"`) {
		t.Fatalf("u did not take it off the list: %s", h.file())
	}
}

// The rename confirm's verb agrees with its count.
func TestSettingsRenameConfirmGrammar(t *testing.T) {
	h := tracksHarness(t, `{"tracks":["SWE","Platform"]}`)
	h.tm.Type("j")
	h.tm.Type("r")
	for range "Platform" {
		h.tm.Type("backspace")
	}
	h.tm.Type("Infra")
	h.tm.Type("enter")
	requireAll(t, squash(h.tm.plain()), "1 Application changes its Track in meta.yml")
}

// Help names the section keys without wrapping a bracket in brackets.
func TestSettingsHelpNamesTheSectionKeysPlainly(t *testing.T) {
	view := chipped(runKeysSized(t, fixtureCopy(t, nil), 120, 40, "6", "?"))
	if strings.Contains(view, "[[") {
		t.Fatalf("help nests brackets:\n%s", view)
	}
	requireAll(t, view, "[←→]", "section")
}
