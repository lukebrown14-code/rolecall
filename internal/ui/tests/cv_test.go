package ui_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

const cvPDF = "Alex Quinn Resume.pdf"

// cvHarness is a buildHarness on a Workspace with two CVs, an imported resume beside
// them, and a downloads folder; external runs (the reveal) are recorded, not started.
func newCVHarness(t *testing.T, built bool) (*buildHarness, string, *[][]string) {
	t.Helper()
	downloads := filepath.Join(t.TempDir(), "Downloads")
	var runs [][]string
	h := newBuildHarness(t, ui.WithPlugin("/plugin"), ui.WithDownloads(downloads),
		ui.WithExternalRunner(func(c *exec.Cmd, done tea.ExecCallback) tea.Cmd {
			runs = append(runs, c.Args)
			return func() tea.Msg { return done(nil) }
		}))
	writeFile(t, filepath.Join(h.root, "cv", "general.md"), "# Alex Quinn\n\n## Key Skills\n\nGo, Python\n")
	writeFile(t, filepath.Join(h.root, "cv", "master.md"), "# Alex Quinn\n\nEvery role.\n")
	writeFile(t, filepath.Join(h.root, "cv", "imported-resume.pdf"), "%PDF-1.4\n")
	if built {
		writeCVPDF(t, h.root, time.Now().Add(time.Minute))
	}
	return h, downloads, &runs
}

// writeCVPDF is what `build.py --cv general` leaves behind, stamped at.
func writeCVPDF(t *testing.T, root string, at time.Time) {
	t.Helper()
	path := filepath.Join(root, "cv", "out", "general", cvPDF)
	writeFile(t, path, "%PDF-1.4 general\n")
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
}

// selectGeneral opens the Profile and selects general.md, the second-last entry.
func (h *buildHarness) selectGeneral() {
	h.key("5")
	h.key("G")
	h.key("k")
}

// listRowHas reports whether the Profile list row naming name ends with mark, before
// the divider.
func listRowHas(view, name, mark string) bool {
	for _, line := range strings.Split(view, "\n") {
		list, _, divided := strings.Cut(line, "│")
		if divided && strings.Contains(list, name) {
			return strings.HasSuffix(strings.TrimSpace(list), mark)
		}
	}
	return false
}

func TestProfileListsEachCVBelowItsOwnRule(t *testing.T) {
	h, _, _ := newCVHarness(t, true)
	h.key("5")
	view := h.plain()
	if !strings.Contains(view, "CVs ─") || !listRowHas(view, "general.md", "✓") || !listRowHas(view, "master.md", "○") {
		t.Fatalf("Profile lacks the CVs and their marks:\n%s", view)
	}
	if strings.Contains(view, "imported-resume") {
		t.Fatalf("an imported resume is not a CV:\n%s", view)
	}
	if strings.Index(view, "general.md") > strings.Index(view, "master.md") {
		t.Fatalf("general.md is not listed first:\n%s", view)
	}
}

func TestProfileShowsTheSelectedCVAndItsPDF(t *testing.T) {
	h, _, _ := newCVHarness(t, true)
	h.selectGeneral()
	h.wait("CV", "general.md", "PDF", cvPDF, "d download", "b build", "Key Skills")
}

func TestCVNotBuiltOffersBuildNotDownload(t *testing.T) {
	h, downloads, _ := newCVHarness(t, false)
	h.selectGeneral()
	h.wait("not built yet", "b build")
	if strings.Contains(h.plain(), "d download") {
		t.Fatalf("d offered with no PDF:\n%s", h.plain())
	}
	h.key("d")
	h.wait("No PDF yet")
	if _, err := os.Stat(downloads); !os.IsNotExist(err) {
		t.Fatalf("d with no PDF touched the downloads folder: %v", err)
	}
}

func TestCVEditedAfterItsBuildIsMarkedStale(t *testing.T) {
	h, _, _ := newCVHarness(t, false)
	writeCVPDF(t, h.root, time.Now().Add(-time.Hour))
	h.selectGeneral()
	h.wait(cvPDF+" is older than general.md", "b rebuild")
	if !listRowHas(h.plain(), "general.md", "⚠") {
		t.Fatalf("general.md is not marked stale:\n%s", h.plain())
	}
}

func TestBOnACVRunsBuildWithCVAndReportsThePDF(t *testing.T) {
	h, _, _ := newCVHarness(t, false)
	h.fake.run = func(string) error {
		writeCVPDF(t, h.root, time.Now().Add(time.Minute))
		return nil
	}
	h.selectGeneral()
	h.key("b")
	if len(h.fake.calls) != 1 {
		t.Fatalf("calls = %v", h.fake.calls)
	}
	want := []string{"python3", filepath.Join("/plugin", "scripts", "build.py"), "--cv", "general", "--data", h.root}
	if strings.Join(h.fake.calls[0], "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("build run = %q, want %q", h.fake.calls[0], want)
	}
	h.wait("Built "+cvPDF, "d download")
	if !listRowHas(h.plain(), "general.md", "✓") {
		t.Fatalf("general.md is not marked built:\n%s", h.plain())
	}
}

func TestCVBuildFailureShowsTheBuildOutput(t *testing.T) {
	h, _, _ := newCVHarness(t, false)
	h.fake.output = "cv/general.md has no '## Key Skills' block."
	h.fake.run = func(string) error { return os.ErrInvalid }
	h.selectGeneral()
	h.key("b")
	h.wait("Build failed", "general.md", "has no '## Key Skills' block")
}

func TestDOnACVCopiesItsPDFAndRevealsDownloads(t *testing.T) {
	h, downloads, runs := newCVHarness(t, true)
	h.selectGeneral()
	h.key("d")
	got, err := os.ReadFile(filepath.Join(downloads, cvPDF))
	if err != nil || string(got) != "%PDF-1.4 general\n" {
		t.Fatalf("downloaded %q, %v", got, err)
	}
	h.wait("Saved 1 PDF → ")
	if len(*runs) != 1 || strings.Join((*runs)[0], "\x00") != strings.Join(expectedOpen(downloads, true), "\x00") {
		t.Fatalf("reveal runs = %q", *runs)
	}
}

func TestDOnACVAsksBeforeOverwriting(t *testing.T) {
	h, downloads, _ := newCVHarness(t, true)
	writeFile(t, filepath.Join(downloads, cvPDF), "older")
	h.selectGeneral()
	h.key("d")
	h.wait("File exists", cvPDF)
	h.key("y")
	if got, _ := os.ReadFile(filepath.Join(downloads, cvPDF)); string(got) != "%PDF-1.4 general\n" {
		t.Fatalf("y did not overwrite: %q", got)
	}
}
