package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// The Leads screen's Filtered view holds the Lead a Search rule dismissed, with the
// reason; restoring it overrules the rule, and u puts the rule back.
func TestFilteredViewRestoresAndUndoPutsTheRuleBack(t *testing.T) {
	root := fixtureCopy(t, nil)
	d := newDriver(t, root, fixtureClock, 120, 40)
	d.Type("1")
	d.Type("s")
	d.Type("s")
	requireAll(t, d.plain(), "[Filtered 1]", "Backend Engineer", "too old")
	d.Type("enter")
	// The note's age is the day the rule fired, and says so beside today's POSTED age.
	requireAll(t, squash(d.plain()), "FILTERED too old · as of 24 Sep: posted 2026-08-01, 54 days ago;", "limit 30", "SOURCE linkedin",
		"POSTED 2026-08-01 · 61d ago", `site said "1mo ago" on 24 Sep`)
	d.Type("esc")
	d.Type("m")
	d.Type("enter") // Dismissed offers its previous Status: Dismissed was its first, so Lead
	meta := filepath.Join(root, "jobs", "SWE", "filtered-expired", "meta.yml")
	b, _ := os.ReadFile(meta)
	if !strings.Contains(string(b), "filter_overruled: true") || strings.Contains(string(b), "filter_reason") {
		t.Fatalf("restore did not overrule the rule:\n%s\n%s", b, d.plain())
	}
	d.Type("u")
	b, _ = os.ReadFile(meta)
	if strings.Contains(string(b), "filter_overruled") || !strings.Contains(string(b), `filter_reason: "expired"`) ||
		!strings.Contains(string(b), "54 days ago") {
		t.Fatalf("undo did not put the rule back:\n%s", b)
	}
	requireAll(t, d.plain(), "Backend Engineer")
}

// A Lead near the max posting age says how long it has left, in the row and the detail.
func TestLeadShowsPostingAgeAndExpiryWarning(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 140, 40)
	d.Type("1")
	d.Type("/Fern")
	d.Type("enter")
	d.Type("enter")
	requireAll(t, squash(d.plain()), "POSTED 2026-09-04 · 27d ago · site said \"27d ago\"", "⚠ expires in 3 days", "APPLICANTS 41", "SOURCE greenhouse")
}

// Rules rows show each Search rule; enter edits one, the save names the file and u undoes it.
func TestSettingsRulesEditAndUndo(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"search_rules":{"max_posting_age_days":30}}`, nil)
	h.tm.Type("6")
	h.tm.Type("]")
	requireAll(t, squash(h.tm.plain()), "Max posting age 30 days workspace.json", "Expiry warning 5 days default")
	h.tm.Type("j")
	h.tm.Type("j")
	h.tm.Type("enter")
	h.tm.Type("linkedin 100, seek 50")
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Saved Applicant caps: linkedin 100, seek 50 · workspace.json")
	if f := h.file(); !strings.Contains(f, `"linkedin": 100`) || !strings.Contains(f, `"max_posting_age_days": 30`) {
		t.Fatalf("workspace.json:\n%s", f)
	}
	h.tm.Type("u")
	if f := h.file(); strings.Contains(f, "linkedin") {
		t.Fatalf("undo left the caps:\n%s", f)
	}
	h.tm.Type("enter")
	h.tm.Type("linkedin")
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "write each cap as a")
}

// Sources rows list the registry; o turns one off and u back on, enter edits it; the add form writes
// a new entry; c runs the Sweep's check of that Source and reports it.
func TestSettingsSourcesToggleAddAndCheck(t *testing.T) {
	var ran []string
	runner := func(c *exec.Cmd, done func(ui.CapturedResult) tea.Msg) tea.Cmd {
		ran = c.Args
		return func() tea.Msg {
			return done(ui.CapturedResult{Stdout: `{"name":"greenhouse","ok":true,"count":41,"access":"works","note":""}` + "\n"})
		}
	}
	root := fixtureCopy(t, nil)
	d := newDriver(t, root, fixtureClock, 140, 40, ui.WithPlugin("/opt/plugin"), ui.WithCapturedRunner(runner))
	d.Type("6")
	d.Type("]")
	d.Type("]")
	requireAll(t, squash(d.plain()), "greenhouse api · greenhouse · 2 targets · works", "linkedin browser · login wall",
		"company-careers manual · unknown · disabled", "+ add a Source")
	if strings.Contains(squash(d.plain()), "greenhouse api · greenhouse · 2 targets · works workspace.json") {
		t.Fatalf("a Source row repeats its file:\n%s", d.plain())
	}
	requireAll(t, keyLineOf(d.View()), "enter edit", "o turn off", "c check")
	d.Type("c")
	if strings.Join(ran, " ") != "python3 /opt/plugin/scripts/sweep.py --data "+root+" --test-source greenhouse" {
		t.Fatalf("ran %q", ran)
	}
	requireAll(t, d.plain(), "greenhouse: works · 41 jobs")
	d.Type("enter") // enter opens the Source's form; it never turns the Source off
	requireAll(t, d.plain(), "Edit Source greenhouse")
	d.Type("esc")
	if strings.Contains(squash(d.plain()), "2 targets · works · disabled") {
		t.Fatalf("enter turned greenhouse off:\n%s", d.plain())
	}
	d.Type("o")
	requireAll(t, squash(d.plain()), "Turned off greenhouse", "2 targets · works · disabled")
	d.Type("u")
	if b, _ := os.ReadFile(filepath.Join(root, "workspace.json")); !strings.Contains(string(b), `"enabled": true`) {
		t.Fatalf("undo did not turn greenhouse back on:\n%s", b)
	}
	d.Type("G")
	d.Type("enter")
	d.Type("seek")
	d.Type("ctrl+s")
	requireAll(t, squash(d.plain()), "Added Source seek", "seek browser · unknown")
}

// c with no Plugin set says where to set one.
func TestSourceCheckNeedsAPlugin(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 140, 40)
	d.Type("6")
	d.Type("]")
	d.Type("]")
	d.Type("c")
	requireAll(t, d.plain(), "No Plugin set")
}

// enter on a Source opens its form with its values; a save rewrites it in place and u
// puts it back as it was.
func TestSettingsEditSourceSavesAndUndoes(t *testing.T) {
	root := fixtureCopy(t, nil)
	d := newDriver(t, root, fixtureClock, 140, 40)
	d.Type("6")
	d.Type("]")
	d.Type("]")
	d.Type("j") // linkedin
	d.Type("enter")
	requireAll(t, d.plain(), "Edit Source linkedin")
	for range 3 {
		d.Type("enter") // Name, Type, Platform to Targets
	}
	d.Type("enter") // URL
	d.Type("ctrl+u")
	d.Type("https://example.test/jobs")
	d.Type("ctrl+s")
	requireAll(t, squash(d.plain()), "Saved Source linkedin · workspace.json")
	ws := func() string { b, _ := os.ReadFile(filepath.Join(root, "workspace.json")); return string(b) }
	if !strings.Contains(ws(), "https://example.test/jobs") {
		t.Fatalf("not saved:\n%s", ws())
	}
	d.Type("u")
	requireAll(t, d.plain(), "Undone · Source linkedin as it was")
	if strings.Contains(ws(), "https://example.test/jobs") {
		t.Fatalf("u did not put the Source back:\n%s", ws())
	}
}
