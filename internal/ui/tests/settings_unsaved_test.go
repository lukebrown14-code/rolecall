package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui"

	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
)

// moveTo presses j until the field cursor marks the row with label.
func moveTo(t *testing.T, d *driver, label string) {
	t.Helper()
	for range 12 {
		if strings.Contains(d.plain(), "▌ "+label) {
			return
		}
		d.Type("j")
	}
	t.Fatalf("cursor never reached %q:\n%s", label, d.plain())
}

func editFollowUp(t *testing.T, d *driver) {
	t.Helper()
	d.Type("6")
	d.Type("]")
	d.Type("]")
	d.Type("]")
	d.Type("]")
	moveTo(t, d, "Follow-up")
	d.Type("enter")
	d.Type("10")
	d.Type("enter")
}

// ctrlC presses ctrl+c and reports whether the program quit.
func ctrlC(t *testing.T, d *driver) bool {
	t.Helper()
	next, cmd := d.m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	d.m = next
	if cmd == nil {
		return false
	}
	_, quit := cmd().(tea.QuitMsg)
	return quit
}

func TestSettingsBracketsAndArrowsMoveBetweenSections(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	requireAll(t, d.plain(), "[Search]")
	d.Type("]")
	requireAll(t, d.plain(), "[Rules]")
	d.Type("[")
	d.Type("[")
	requireAll(t, d.plain(), "[This machine]")
	d.Type("right")
	requireAll(t, d.plain(), "[Search]")
	d.Type("left")
	requireAll(t, d.plain(), "[This machine]")
}

// Enter writes the change at once and says what and where; u puts it back.
func TestSettingsEnterSavesAtOnceAndUUndoes(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	editFollowUp(t, d)
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "follow_up_days = 10\n") {
		t.Fatalf("enter did not write the file: %q", b)
	}
	v := d.plain()
	requireAll(t, v, "Saved Follow-up due after: 10 · config.toml", "10 days", "u undo")
	for _, gone := range []string{"unsaved", "ctrl+s", "ctrl+z", "was 14"} {
		if strings.Contains(v, gone) {
			t.Fatalf("staged-save leftover %q:\n%s", gone, v)
		}
	}
	d.Type("u")
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("u did not restore the file: %q", b)
	}
	requireAll(t, d.plain(), "14 days")
}

// No group offers a save or revert key: every change is already saved.
func TestSettingsKeyLineHasNoStagedSaveKeys(t *testing.T) {
	for _, sz := range [][2]int{{120, 36}, {80, 24}, {60, 16}} {
		for _, keys := range [][]string{{"6"}, {"6", "]", "]", "]"}, {"6", "]", "]", "]", "]"}, {"6", "[", "enter", "x"}} {
			help := chipped(runKeysSized(t, fixtureCopy(t, nil), sz[0], sz[1], append(keys, "?")...))
			line := lastLine(t, sz[0], sz[1], keys...)
			for _, gone := range []string{"ctrl+s", "ctrl+z", "revert"} {
				if strings.Contains(line, gone) || strings.Contains(help, "["+gone) {
					t.Errorf("%dx%d %q: offers %q: %q", sz[0], sz[1], keys, gone, line)
				}
			}
		}
	}
}

func TestSettingsEscCancelsAnInlineEdit(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	d.Type("6")
	d.Type("]")
	d.Type("]")
	d.Type("]")
	d.Type("]")
	moveTo(t, d, "Follow-up")
	d.Type("enter")
	d.Type("9")
	d.Type("esc")
	v := d.plain()
	if strings.Contains(v, "Discard") || !strings.Contains(v, "14 days") {
		t.Fatalf("esc did not cancel:\n%s", v)
	}
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("file changed: %q", b)
	}
}

// ctrl+c over typed text asks first, like esc on a form; y drops the text and keeps the
// program running, and a second ctrl+c quits.
func TestSettingsCtrlCOverTypedTextAsksBeforeDropping(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, settings.GroupMachine, 0)
	d.Type("enter")
	d.Type("nano")
	if ctrlC(t, d) {
		t.Fatal("ctrl+c quit over typed text")
	}
	requireAll(t, d.plain(), "Discard changes?", "y discard", "keep editing")
	d.Type("n")
	requireAll(t, d.plain(), "nano▏")
	if ctrlC(t, d) {
		t.Fatal("ctrl+c quit over typed text")
	}
	d.Type("y")
	if strings.Contains(d.plain(), "▏") {
		t.Fatalf("y kept the field open:\n%s", d.plain())
	}
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("file changed: %q", b)
	}
	if !ctrlC(t, d) {
		t.Fatal("ctrl+c with nothing typed did not quit")
	}
}

func TestSettingsCtrlCInAPopupWithTypedTextAsks(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	d.Type("enter")
	d.Type("x")
	if ctrlC(t, d) {
		t.Fatal("ctrl+c quit over a changed Search area form")
	}
	requireAll(t, d.plain(), "Discard changes?")
	if !ctrlC(t, d) {
		t.Fatal("a second ctrl+c did not quit")
	}
}

func TestSettingsCtrlCWithAnUntouchedFieldQuits(t *testing.T) {
	d, _ := machineDriver(t, 120)
	selectRow(d, settings.GroupMachine, 0)
	d.Type("enter")
	if !ctrlC(t, d) {
		t.Fatal("ctrl+c did not quit with nothing typed")
	}
}

// An unset value opens empty, its default as a placeholder, so typing never appends to it.
func TestSettingsUnsetValueOpensEmptyWithTheDefaultShown(t *testing.T) {
	home := t.TempDir()
	cfg := config.Path(home)
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte("# empty\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "")
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 120, 40, ui.WithConfigHome(home))
	selectRow(d, settings.GroupMachine, 0)
	requireAll(t, squash(d.plain()), "Editor vi default")
	d.Type("enter")
	requireAll(t, d.plain(), "▏vi")
	d.Type("nvim")
	requireAll(t, d.plain(), "nvim▏")
	if strings.Contains(d.plain(), "vinvim") {
		t.Fatalf("typing appended to the default:\n%s", d.plain())
	}
	d.Type("enter")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), `editor = "nvim"`) {
		t.Fatalf("config.toml: %q", b)
	}
}

// A set value opens selected: the first key typed replaces it.
func TestSettingsSetValueIsReplacedByTyping(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, settings.GroupMachine, 0)
	d.Type("enter")
	d.Type("hx")
	d.Type("enter")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), `editor = "hx"`) {
		t.Fatalf("config.toml: %q", b)
	}
	requireAll(t, d.plain(), "Saved Editor: hx · config.toml")
}

func TestSettingsTabsShowEveryGroupWhenTheyFit(t *testing.T) {
	for _, w := range []int{80, 120} {
		lines := settingsAt(t, w, 24)
		if l := rowWith(lines, "SETTINGS"); !strings.Contains(l, "[Search]  Rules  Sources  Documents  Tracking  This machine") {
			t.Errorf("%d: tabs %q", w, l)
		}
	}
	for _, w := range []int{56, 60, 72} {
		if l := rowWith(settingsAt(t, w, 16), "SETTINGS"); !strings.Contains(l, "[Search] 1 of 6") {
			t.Errorf("%d: tabs %q", w, l)
		}
	}
}

// g and G go to the first and last row, as in lists.
func TestSettingsGAndShiftGGoToTheEnds(t *testing.T) {
	lines := settingsAt(t, 120, 36, "]", "]", "]", "]", "G")
	if l := rowWith(lines, "Follow-up due after"); !strings.Contains(l, "▌") {
		t.Errorf("G: %q", l)
	}
	lines = settingsAt(t, 120, 36, "]", "]", "]", "]", "G", "g")
	if l := rowWith(lines, "▌ SWE"); l == "" {
		t.Errorf("g:\n%s", strings.Join(lines, "\n"))
	}
}

// Settings reached with 6 returns on esc to where the Owner was, as it does after ",".
func TestEscInSettingsOpenedByDigitReturns(t *testing.T) {
	v := chipped(runKeys(t, fixtureCopy(t, nil), 120, "3", "6", "esc"))
	if !strings.Contains(v, "[3 Applied") {
		t.Fatalf("esc did not return to Applied:\n%s", v)
	}
	if line := lastLine(t, 120, 36, "3", "6"); !strings.Contains(line, "esc back") {
		t.Fatalf("key line lacks esc back: %q", line)
	}
}

func TestSettingsInlineFieldIsNoWiderThanThirtyCells(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	for range 5 {
		d.Type("]")
	}
	moveTo(t, d, "Editor")
	d.Type("enter")
	d.Type(strings.Repeat("x", 80))
	for _, line := range strings.Split(d.plain(), "\n") {
		if strings.Contains(line, "▏") {
			if n := strings.Count(line, "x"); n > 30 {
				t.Fatalf("inline field shows %d cells:\n%s", n, line)
			}
			return
		}
	}
	t.Fatalf("no inline field shown:\n%s", d.plain())
}

func TestSettingsTrackCountsSplitActiveAndClosed(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	d.Type("]")
	d.Type("]")
	d.Type("]")
	d.Type("]")
	requireAll(t, squash(d.plain()), "applications · ", " active")
}

// Undoing the save of a setting that was unset names what it falls back to, as the row
// showed it, not "not set".
func TestSettingsUndoNamesTheFallback(t *testing.T) {
	h := newSourcesHarness(t, 140, "", nil)
	editFollowUp(t, h.d)
	h.d.Type("u")
	requireAll(t, h.d.plain(), "Follow-up due after back to the default (14)")
}
