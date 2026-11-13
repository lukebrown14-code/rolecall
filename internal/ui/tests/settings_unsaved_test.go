package ui_test

import (
	"os"
	"strings"
	"testing"
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
	moveTo(t, d, "Follow-up")
	d.Type("enter")
	for range 3 {
		d.Type("backspace")
	}
	d.Type("10")
	d.Type("enter")
}

func TestSettingsBracketsMoveBetweenSections(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	requireAll(t, d.plain(), "[ Search ]")
	d.Type("]")
	requireAll(t, d.plain(), "[ Documents ]")
	d.Type("[")
	d.Type("[")
	requireAll(t, d.plain(), "[ This machine ]")
}

func TestSettingsEditShowsDirtyAndWasThenCtrlSSaves(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	editFollowUp(t, d)
	requireAll(t, d.plain(), "● 1 unsaved change", "was 14")
	if strings.Contains(d.plain(), "Preview save") {
		t.Fatalf("Preview save still shown:\n%s", d.plain())
	}
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("file written before ctrl+s: %q", b)
	}
	d.Type("ctrl+s")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "follow_up_days = 10\n") {
		t.Fatalf("ctrl+s did not write the file: %q", b)
	}
	v := d.plain()
	requireAll(t, v, "10 days")
	if strings.Contains(v, "unsaved") || strings.Contains(v, "was 14") {
		t.Fatalf("dirty marker left after save:\n%s", v)
	}
}

func TestSettingsCtrlZRevertsTheFieldAndClearsTheMarker(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	editFollowUp(t, d)
	d.Type("ctrl+z")
	v := d.plain()
	requireAll(t, v, "14 days")
	if strings.Contains(v, "unsaved") || strings.Contains(v, "was 14") {
		t.Fatalf("marker left after ctrl+z:\n%s", v)
	}
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("file changed: %q", b)
	}
}

func TestSettingsCtrlZOnTheHeaderRevertsEverything(t *testing.T) {
	d, _ := machineDriver(t, 120)
	editFollowUp(t, d)
	d.Type("]")
	moveTo(t, d, "Editor")
	d.Type("enter")
	d.Type("x")
	d.Type("enter")
	requireAll(t, d.plain(), "● 2 unsaved changes")
	for range 10 {
		d.Type("k")
	}
	d.Type("ctrl+z")
	if strings.Contains(d.plain(), "unsaved") {
		t.Fatalf("ctrl+z on the header kept changes:\n%s", d.plain())
	}
}

func TestSettingsEscCancelsAnInlineEdit(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	d.Type("]")
	d.Type("]")
	moveTo(t, d, "Follow-up")
	d.Type("enter")
	d.Type("9")
	d.Type("esc")
	v := d.plain()
	if strings.Contains(v, "unsaved") || !strings.Contains(v, "14 days") {
		t.Fatalf("esc did not cancel:\n%s", v)
	}
}

func TestSettingsScreenDigitWhileDirtyAsksAndDiscardLeavesTheFile(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	editFollowUp(t, d)
	d.Type("1")
	requireAll(t, d.plain(), "Unsaved changes", "save", "discard", "cancel")
	d.Type("d")
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("discard changed the file: %q", b)
	}
	if strings.Contains(d.plain(), "[6 Settings]") {
		t.Fatalf("still on Settings after discard:\n%s", d.plain())
	}
}

func TestSettingsEscWhileDirtyAsksAndSaveWrites(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	d.Type(",") // opened with a comma, so esc has a screen to go back to
	d.Type("]")
	d.Type("]")
	moveTo(t, d, "Follow-up")
	d.Type("enter")
	d.Type("backspace")
	d.Type("backspace")
	d.Type("10")
	d.Type("enter")
	d.Type("esc")
	requireAll(t, d.plain(), "Unsaved changes")
	d.Type("s")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "follow_up_days = 10\n") {
		t.Fatalf("save did not write: %q", b)
	}
}

func TestSettingsInlineFieldIsNoWiderThanThirtyCells(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("6")
	for range 3 {
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
	requireAll(t, squash(d.plain()), "applications · ", " active")
}
