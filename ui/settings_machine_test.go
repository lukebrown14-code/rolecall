package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/config"
	"github.com/lukebrown14-code/rolecall/ui"
)

const machineConfig = "# my config\nworkspace = \"/w\" # keep\n\n# my editor\neditor = \"vim\"\nfollow_up_days = 14\n"

// machineDriver opens Settings with a config.toml under a temp home and no harness or plugin.
func machineDriver(t *testing.T, width int) (*driver, string) {
	t.Helper()
	home := t.TempDir()
	cfg := config.Path(home)
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(machineConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	root := fixtureCopy(t, func(root string) {
		writeFile(t, filepath.Join(root, "jobs", "SWE", "status-draft", "resume.md"), "Draft text") // so b has something to build
	})
	d := newDriver(t, root, fixtureClock, width, 40, ui.WithConfigHome(home),
		ui.WithEditor("vim"), ui.WithFollowUpDays(14))
	return d, cfg
}

// selectRow opens Settings, picks the group by index and the row by index.
func selectRow(d *driver, group, row int) {
	d.Type("6")
	for range group {
		d.Type("j")
	}
	d.Type("tab")
	for range row {
		d.Type("j")
	}
}

func TestSettingsEditEditorSavesAndKeepsComments(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 3, 1)
	d.Type("enter")
	for range 3 {
		d.Type("backspace")
	}
	d.Type("nano")
	d.Type("enter")
	requireAll(t, d.plain(), "Editor", "nano")
	b, _ := os.ReadFile(cfg)
	want := "# my config\nworkspace = \"/w\" # keep\n\n# my editor\neditor = \"nano\"\nfollow_up_days = 14\n"
	if string(b) != want {
		t.Fatalf("config.toml:\n%s", b)
	}
}

func TestSettingsEscCancelsAnEditWithoutWriting(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 3, 1)
	d.Type("enter")
	d.Type("zzz")
	d.Type("esc")
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("written:\n%s", b)
	}
	requireAll(t, d.plain(), "vim")
}

func TestSettingsFollowUpDaysRefusedOutsideOneToNinety(t *testing.T) {
	for _, bad := range []string{"0", "91", "abc", ""} {
		d, cfg := machineDriver(t, 120)
		selectRow(d, 2, 1)
		d.Type("enter")
		for range 3 {
			d.Type("backspace")
		}
		d.Type(bad)
		d.Type("enter")
		requireAll(t, d.plain(), "1 to 90")
		if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
			t.Fatalf("%q written:\n%s", bad, b)
		}
	}
}

func TestSettingsFollowUpDaysSavesAndAppliesAtOnce(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 2, 1)
	d.Type("enter")
	d.Type("backspace")
	d.Type("backspace")
	d.Type("30")
	d.Type("enter")
	requireAll(t, d.plain(), "Follow-up days", "30")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "follow_up_days = 30\n") {
		t.Fatalf("config.toml:\n%s", b)
	}
	d.Type("4")
	requireAll(t, d.plain(), "Applied within 30 days left out")
}

func TestSettingsHarnessAndPluginShowKeysWithoutRestart(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("1")
	if v := d.plain(); strings.Contains(v, "assess fit") || strings.Contains(v, "build") {
		t.Fatalf("keys shown before they are set:\n%s", v)
	}
	selectRow(d, 3, 3)
	d.Type("enter")
	d.Type("claude {prompt}")
	d.Type("enter")
	d.Type("j")
	d.Type("enter")
	d.Type("/opt/plugin")
	d.Type("enter")
	d.Type("2")
	d.Type("j")
	d.Type("?")
	requireAll(t, d.plain(), "assess fit", "build")
}

func TestSettingsASCIIToggleSwitchesGlyphsAndSaves(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 3, 5)
	requireAll(t, d.plain(), "ASCII glyphs", "No")
	d.Type("enter")
	requireAll(t, d.plain(), "ASCII glyphs", "Yes")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "ascii = true\n") {
		t.Fatalf("config.toml:\n%s", b)
	}
	for _, r := range ansi.Strip(d.plain()) {
		if r > 0x7e {
			t.Fatalf("non-ASCII glyph %q with ascii on:\n%s", r, d.plain())
		}
	}
}

func TestSettingsWorkspaceRowIsReadOnly(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, 3, 0)
	d.Type("enter")
	d.Type("x")
	d.Type("enter")
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("written:\n%s", b)
	}
}
