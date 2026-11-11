package ui_test

import (
	tea "charm.land/bubbletea/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lukebrown14-code/rolecall/ui"
)

// downloadHarness is a detailHarness whose downloads folder is a temp directory.
func newDownloadHarness(t *testing.T, setup func(root, downloads string)) (*detailHarness, string) {
	t.Helper()
	downloads := filepath.Join(t.TempDir(), "Downloads")
	h := newDetailHarness(t, func(root string) {
		if setup != nil {
			setup(root, downloads)
		}
	}, ui.WithDownloads(downloads))
	return h, downloads
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestDownloadCopiesEveryPDFAndRevealsFolder(t *testing.T) {
	h, downloads := newDownloadHarness(t, nil)
	h.open("3", 3, "docs-ready")
	h.tm.Type("d")
	run := h.lastRun()
	if len(run) != 3 || run[0] != "open" || run[1] != "-R" || run[2] != downloads {
		t.Fatalf("reveal run: %q", run)
	}
	for _, name := range []string{"Owner Resume Role.pdf", "Owner Role Cover Letter.pdf"} {
		want := readFile(t, filepath.Join(h.folder("docs-ready"), "out", name))
		if got := readFile(t, filepath.Join(downloads, name)); got != want {
			t.Fatalf("%s copied differently", name)
		}
	}
	h.show("2 PDFs → " + downloads)
}

// clash puts different files under both PDF names in the downloads folder.
func clash(root, downloads string) {
	os.MkdirAll(downloads, 0o755)
	for _, name := range []string{"Owner Resume Role.pdf", "Owner Role Cover Letter.pdf"} {
		os.WriteFile(filepath.Join(downloads, name), []byte("older "+name), 0o600)
	}
}

func TestDownloadClashPromptsAndNSkipsThatFile(t *testing.T) {
	h, downloads := newDownloadHarness(t, clash)
	h.open("3", 3, "docs-ready")
	h.tm.Type("d")
	h.wait("Overwrite Owner Resume Role.pdf? y/n")
	h.tm.Type("n") // keys are handled in order, so the second prompt is answered next
	h.tm.Type("y")
	h.lastRun()
	if got := readFile(t, filepath.Join(downloads, "Owner Resume Role.pdf")); got != "older Owner Resume Role.pdf" {
		t.Fatalf("skipped file was overwritten: %q", got)
	}
	want := readFile(t, filepath.Join(h.folder("docs-ready"), "out", "Owner Role Cover Letter.pdf"))
	if got := readFile(t, filepath.Join(downloads, "Owner Role Cover Letter.pdf")); got != want {
		t.Fatalf("confirmed file was not overwritten: %q", got)
	}
	h.show("1 PDF → " + downloads)
}

func TestDownloadPromptIsNotRevealedUntilAnswered(t *testing.T) {
	h, downloads := newDownloadHarness(t, clash)
	h.open("3", 3, "docs-ready")
	h.tm.Type("d")
	h.wait("Overwrite Owner Resume Role.pdf? y/n")
	h.tm.Type("j") // other keys do not answer the prompt
	h.show("Overwrite Owner Resume Role.pdf? y/n")
	if got := readFile(t, filepath.Join(downloads, "Owner Resume Role.pdf")); got != "older Owner Resume Role.pdf" {
		t.Fatalf("overwritten before answering: %q", got)
	}
}

func TestDownloadIsHiddenAndInertWithoutPDFs(t *testing.T) {
	h, downloads := newDownloadHarness(t, nil)
	h.open("1", 0, "Fern")
	v := h.show("Fern Systems")
	if strings.Contains(v, "d download") {
		t.Fatalf("help offers download with nothing built:\n%s", v)
	}
	h2, downloads2 := newDownloadHarness(t, nil)
	h2.tm.Type("1")
	h2.tm.Type("/Fern")
	h2.key(tea.KeyEnter)
	h2.tm.Type("d")
	v = h2.show("Fern")
	if strings.Contains(v, "d download") || strings.Contains(v, "PDF") {
		t.Fatalf("d was not inert:\n%s", v)
	}
	if len(h2.runs) != 0 {
		t.Fatalf("ran %q", h2.runs)
	}
	for _, dir := range []string{downloads, downloads2} {
		if _, err := os.Stat(dir); err == nil {
			t.Fatalf("%s was created", dir)
		}
	}
	// From the detail view too.
	h3, downloads3 := newDownloadHarness(t, nil)
	h3.open("1", 0, "Fern")
	h3.tm.Type("d")
	h3.show("Fern Systems · Platform Engineer")
	if _, err := os.Stat(downloads3); err == nil || len(h3.runs) != 0 {
		t.Fatalf("d acted in the detail view with no PDFs: %v %q", err, h3.runs)
	}
}

func TestHelpBarOffersDownloadWhenRowHasPDFs(t *testing.T) {
	table, _ := newDownloadHarness(t, nil)
	table.tm.Type("3")
	for i := 0; i < 3; i++ {
		table.key(tea.KeyTab)
	}
	table.tm.Type("/docs-ready")
	table.key(tea.KeyEnter)
	table.show("d download")
	detail, _ := newDownloadHarness(t, nil)
	detail.open("3", 3, "docs-ready")
	detail.show("d download")
}

func TestDownloadWorksFromDetailViewDocsTab(t *testing.T) {
	h, downloads := newDownloadHarness(t, nil)
	h.open("3", 3, "docs-ready")
	h.key(tea.KeyRight)
	h.key(tea.KeyRight)
	h.key(tea.KeyRight) // Docs tab
	h.tm.Type("d")
	if run := h.lastRun(); len(run) != 3 || run[2] != downloads {
		t.Fatalf("reveal run: %q", run)
	}
	h.show("Docs", "2 PDFs → "+downloads)
	for _, name := range []string{"Owner Resume Role.pdf", "Owner Role Cover Letter.pdf"} {
		readFile(t, filepath.Join(downloads, name))
	}
}

func TestDownloadPromptEscCancelsWithoutReveal(t *testing.T) {
	h, downloads := newDownloadHarness(t, clash)
	h.open("3", 3, "docs-ready")
	h.tm.Type("d")
	h.wait("Overwrite Owner Resume Role.pdf? y/n")
	h.key(tea.KeyEscape)
	v := h.show("Fictional Labs")
	if strings.Contains(v, "Overwrite") || len(h.runs) != 0 {
		t.Fatalf("prompt still open or revealed: %q\n%s", h.runs, v)
	}
	if got := readFile(t, filepath.Join(downloads, "Owner Resume Role.pdf")); got != "older Owner Resume Role.pdf" {
		t.Fatalf("overwritten: %q", got)
	}
}
