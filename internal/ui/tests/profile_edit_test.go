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

// profileHarness drives the Profile screen with a fake editor. The editor records what it
// opened, then rewrites that file, as a real editor would before the TUI resumes.
type profileHarness struct {
	t    *testing.T
	root string
	tm   *driver
	mu   sync.Mutex
	runs [][]string
	edit func(path string)
}

func newProfileHarness(t *testing.T, setup func(root string), edit func(path string)) *profileHarness {
	t.Helper()
	root := fixtureCopy(t, setup)
	h := &profileHarness{t: t, root: root, edit: edit}
	h.tm = newDriver(t, root, fixtureClock, 120, 40,
		ui.WithEditor("myeditor"),
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

// opened is the file the editor was last given.
func (h *profileHarness) opened() string {
	h.t.Helper()
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.runs) == 0 {
		return ""
	}
	run := h.runs[len(h.runs)-1]
	return run[len(run)-1]
}

func (h *profileHarness) launched() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.runs)
}

// rewriteMasterProfile replaces the file the editor is given, keeping the Experience
// heading and adding a new heading above it so the selection has to survive a move.
func rewriteMasterProfile(t *testing.T) func(path string) {
	t.Helper()
	return func(path string) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.Replace(string(b), "## Experience", "## Awards\n\n- One award.\n\n## Experience", 1)
		text = strings.Replace(text, "Moved the team's CI from Jenkins to GitHub Actions.",
			"Moved the team's CI from Jenkins to GitHub Actions on a Friday.", 1)
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProfileEEditsTheSelectedSectionAndKeepsItSelected(t *testing.T) {
	h := newProfileHarness(t, nil, rewriteMasterProfile(t))
	h.tm.Type("5jj")
	h.tm.Type("e")
	if got, want := h.opened(), filepath.Join(h.root, "profile", "master-profile.md"); got != want {
		t.Fatalf("edited %q, want %q", got, want)
	}
	v := h.tm.plain()
	for _, want := range []string{"on a Friday.", "▌ Experience", "Awards", "profile/master-profile.md"} {
		if !strings.Contains(v, want) {
			t.Fatalf("after the editor the screen lacks %q:\n%s", want, v)
		}
	}
	if strings.Contains(v, "▌ Awards") || strings.Contains(v, "One award.") {
		t.Fatalf("the selection moved off Experience:\n%s", v)
	}
}

func TestProfileEEditsTheSelectedOtherFile(t *testing.T) {
	h := newProfileHarness(t, nil, func(path string) {
		if err := os.WriteFile(path, []byte("# Projects\n\n## Tide Tables\n\nNow prints tide tables in colour.\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	h.tm.Type(strings.Join(profileKeysToFile, ""))
	h.tm.Type("e")
	if got, want := h.opened(), filepath.Join(h.root, "profile", "projects.md"); got != want {
		t.Fatalf("edited %q, want %q", got, want)
	}
	v := h.tm.plain()
	for _, want := range []string{"Now prints tide tables in colour.", "▌ projects.md", "profile/projects.md"} {
		if !strings.Contains(v, want) {
			t.Fatalf("after editing projects.md the screen lacks %q:\n%s", want, v)
		}
	}
}

// When the edited section's heading is gone, the selection returns to the first section.
func TestProfileESelectionFallsBackToTheFirstSection(t *testing.T) {
	h := newProfileHarness(t, nil, func(path string) {
		text := "# Master Profile\n\n## Contact\n\n| Field | Value |\n| :-- | :-- |\n" +
			"| Name | Alex Quinn |\n| Email | alex.quinn@example.test |\n\n## Summary\n\nA new summary.\n"
		if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	})
	h.tm.Type("5jj")
	h.tm.Type("e")
	v := h.tm.plain()
	for _, want := range []string{"▌ Contact", "alex.quinn@example.test"} {
		if !strings.Contains(v, want) {
			t.Fatalf("selection did not fall back to Contact (%q):\n%s", want, v)
		}
	}
	if strings.Contains(v, "Experience") {
		t.Fatalf("the removed section is still shown:\n%s", v)
	}
}

func TestProfileRReReadsTheFiles(t *testing.T) {
	h := newProfileHarness(t, nil, nil)
	h.tm.Type("5")
	path := filepath.Join(h.root, "profile", "master-profile.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(string(b)+"\n## Volunteer\n\nCommunity mentoring.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.tm.Type("r")
	v := h.tm.plain()
	for _, want := range []string{"Volunteer", "▌ Contact"} {
		if !strings.Contains(v, want) {
			t.Fatalf("r did not reread the Profile (%q):\n%s", want, v)
		}
	}
}

func TestProfileSwitchingBackReReadsTheFiles(t *testing.T) {
	h := newProfileHarness(t, nil, nil)
	h.tm.Type("5")
	path := filepath.Join(h.root, "profile", "master-profile.md")
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(string(b)+"\n## Volunteer\n\nCommunity mentoring.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	h.tm.Type("15")
	v := h.tm.plain()
	for _, want := range []string{"Volunteer", "▌ Contact"} {
		if !strings.Contains(v, want) {
			t.Fatalf("switching back did not reread the Profile (%q):\n%s", want, v)
		}
	}
}

func TestProfileWithNoProfileEEditsNothing(t *testing.T) {
	h := newProfileHarness(t, func(root string) {
		if err := os.Remove(filepath.Join(root, "profile", "master-profile.md")); err != nil {
			t.Fatal(err)
		}
	}, func(path string) { t.Fatalf("editor launched on %q", path) })
	h.tm.Type("5e")
	if n := h.launched(); n != 0 {
		t.Fatalf("e launched %d external runs with no Profile", n)
	}
	if _, err := os.Stat(filepath.Join(h.root, "profile", "master-profile.md")); !os.IsNotExist(err) {
		t.Fatalf("e created a Profile: %v", err)
	}
}

func TestProfileEKeepsFocus(t *testing.T) {
	h := newProfileHarness(t, nil, rewriteMasterProfile(t))
	h.tm.Type("5jj")
	h.tm.Type("\t")
	h.tm.Type("e")
	v := h.tm.plain()
	for _, want := range []string{"CONTENT", "on a Friday.", "▌ Experience"} {
		if !strings.Contains(v, want) {
			t.Fatalf("after the editor the screen lacks %q:\n%s", want, v)
		}
	}
}

func TestProfileHelpListsEditFile(t *testing.T) {
	view := strings.ToLower(runKeys(t, fixtureCopy(t, nil), 120, "5", "?"))
	if !strings.Contains(view, "edit file") {
		t.Fatalf("help does not list e / edit file:\n%s", view)
	}
}
