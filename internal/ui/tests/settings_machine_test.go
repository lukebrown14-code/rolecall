package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui"

	"github.com/lukebrown14-code/rolecall/internal/ui/settings"
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
		d.Type("]")
	}
	for range row {
		d.Type("j")
	}
}

func TestSettingsEditEditorSavesAndKeepsComments(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, settings.GroupMachine, 0)
	d.Type("enter")
	for range 3 {
		d.Type("backspace")
	}
	d.Type("nano")
	d.Type("enter")
	requireAll(t, d.plain(), "Editor", "nano", "Saved Editor: nano · config.toml")
	b, _ := os.ReadFile(cfg)
	want := "# my config\nworkspace = \"/w\" # keep\n\n# my editor\neditor = \"nano\"\nfollow_up_days = 14\n"
	if string(b) != want {
		t.Fatalf("config.toml:\n%s", b)
	}
}

func TestSettingsRereadsConfigOnEnteringTheScreen(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	if err := os.WriteFile(cfg, []byte(strings.Replace(machineConfig, `"vim"`, `"nano"`, 1)), 0o600); err != nil {
		t.Fatal(err)
	}
	selectRow(d, settings.GroupMachine, 0)
	requireAll(t, d.plain(), "Editor", "nano")
}

func TestSettingsRereadsConfigAfterTheEditorCloses(t *testing.T) {
	home := t.TempDir()
	cfg := config.Path(home)
	if err := os.MkdirAll(filepath.Dir(cfg), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfg, []byte(machineConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	root := fixtureCopy(t, nil)
	d := newDriver(t, root, fixtureClock, 120, 40, ui.WithConfigHome(home), ui.WithEditor("vim"),
		ui.WithExternalRunner(func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
			return func() tea.Msg {
				// the Owner edits config.toml in another window while the editor is open
				err := os.WriteFile(cfg, []byte(strings.Replace(machineConfig, `"vim"`, `"emacs"`, 1)), 0o600)
				return done(err)
			}
		}))
	selectRow(d, settings.GroupMachine, 0)
	requireAll(t, d.plain(), "vim")
	d.Type("e")
	requireAll(t, d.plain(), "emacs")
}

func TestSettingsEscCancelsAnEditWithoutWriting(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, settings.GroupMachine, 0)
	d.Type("enter")
	d.Type("zzz")
	d.Type("esc")
	if v := d.plain(); strings.Contains(v, "Discard changes?") || strings.Contains(v, "unsaved") || strings.Contains(v, "zzz") {
		t.Fatalf("esc keeps the edit:\n%s", v)
	}
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("written:\n%s", b)
	}
	requireAll(t, d.plain(), "vim")
}

func TestSettingsFollowUpDaysRefusedOutsideOneToNinety(t *testing.T) {
	for _, bad := range []string{"0", "91", "abc", ""} {
		d, cfg := machineDriver(t, 120)
		selectRow(d, settings.GroupTracking, 0)
		d.Type("G")
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
	selectRow(d, settings.GroupTracking, 0)
	d.Type("G")
	d.Type("enter")
	d.Type("30")
	d.Type("enter")
	requireAll(t, d.plain(), "Follow-up due after", "30 days")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "follow_up_days = 30\n") {
		t.Fatalf("config.toml:\n%s", b)
	}
	d.Type("4")
	requireAll(t, d.plain(), "applied in the last 30 days")
}

func TestSettingsHarnessAndPluginShowKeysWithoutRestart(t *testing.T) {
	d, _ := machineDriver(t, 120)
	d.Type("1")
	// The detail rows show f and b all along (pressing one names the setting); the key
	// line lists them only once they work.
	lines := strings.Split(strings.TrimRight(d.plain(), "\n"), "\n")
	if keyLine := lines[len(lines)-1]; strings.Contains(keyLine, "assess fit") || strings.Contains(keyLine, "build") {
		t.Fatalf("key line lists keys before they are set: %q", keyLine)
	}
	selectRow(d, settings.GroupMachine, 2)
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
	selectRow(d, settings.GroupMachine, 4)
	requireAll(t, d.plain(), "ASCII glyphs", "No")
	d.Type("enter")
	requireAll(t, d.plain(), "ASCII glyphs", "Yes", "Saved ASCII glyphs: Yes")
	if b, _ := os.ReadFile(cfg); !strings.Contains(string(b), "ascii = true\n") {
		t.Fatalf("config.toml:\n%s", b)
	}
	for _, r := range chipped(d.plain()) {
		if r > 0x7e {
			t.Fatalf("non-ASCII glyph %q with ascii on:\n%s", r, d.plain())
		}
	}
}

// The cursor passes over the read-only Workspace row, so enter never lands on it and does
// nothing: tab and k both stop at the Editor below it.
func TestSettingsWorkspaceRowIsReadOnly(t *testing.T) {
	d, cfg := machineDriver(t, 120)
	selectRow(d, settings.GroupMachine, 0)
	d.Type("k")
	if l := rowWith(strings.Split(d.plain(), "\n"), "Workspace"); strings.Contains(l, "▌") {
		t.Fatalf("cursor landed on the read-only Workspace row: %q\n%s", l, d.plain())
	}
	d.Type("enter") // on the header: nothing opens
	if strings.Contains(d.plain(), "▏") {
		t.Fatalf("enter on the header opened a field:\n%s", d.plain())
	}
	d.Type("j")
	d.Type("enter")
	requireAll(t, d.plain(), "▏")
	d.Type("esc")
	if b, _ := os.ReadFile(cfg); string(b) != machineConfig {
		t.Fatalf("written:\n%s", b)
	}
}
