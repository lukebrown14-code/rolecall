package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// An Application's documents, each with one name everywhere (GLOSSARY): JD, Resume,
// Cover letter, Gaps and PDF. A document is ready or missing, nothing in between; the
// file it lives in is shown, muted, only where the Owner may want to open it.
const (
	docJD          = "JD"
	docResume      = "Resume"
	docCoverLetter = "Cover letter"
	docGaps        = "Gaps"
	docPDF         = "PDF"
	docNotes       = "Notes"
	docFit         = "Fit"
)

// appName is an Application's title as a message or breadcrumb names it: quoted when it
// would not read as a name on its own (a bare slug such as docs-ready, or stray spaces),
// and never blank.
func appName(title string) string {
	t := strings.TrimSpace(title)
	switch {
	case t == "":
		return untitled
	case t != title, !strings.Contains(t, " ") && strings.ContainsAny(t, "-_/."):
		return "“" + t + "”"
	}
	return t
}

// The source files the documents live in (SCHEMA), beside coverLetterFile.
const (
	jdFile     = "jd.md"
	resumeFile = "resume.md"
	gapsFile   = "gaps.md"
	notesFile  = "notes.md"
	fitFile    = "fit.md"
)

// The two document states.
const (
	docReady   = "ready"
	docMissing = "missing"
)

// docNames names each source file as the Owner knows it.
var docNames = map[string]string{jdFile: docJD, resumeFile: docResume, coverLetterFile: docCoverLetter,
	gapsFile: docGaps, notesFile: docNotes, fitFile: docFit}

// docName is a source file's document name, or the file name for anything else.
func docName(file string) string {
	if n, ok := docNames[file]; ok {
		return n
	}
	return file
}

// docChip is one document in the set that makes an Application sendable.
type docChip struct {
	name, file string
	ready      bool
}

// docChips is the set that makes an Application sendable: JD, Resume, Cover letter
// (unless it is Resume only) and PDF. The PDF is ready once the build has made every
// PDF the Application needs (SCHEMA Ready).
func docChips(a store.Application) []docChip {
	chips := []docChip{{docJD, jdFile, a.JDHasText}, {docResume, resumeFile, a.ResumeHasText}}
	if !a.ResumeOnly {
		chips = append(chips, docChip{docCoverLetter, coverLetterFile, a.CoverLetterHasText})
	}
	return append(chips, docChip{docPDF, "out/", a.Documents == store.DocsReady})
}

// readyCount is how many of the Application's documents are ready, of how many.
func readyCount(a store.Application) (int, int) {
	chips := docChips(a)
	n := 0
	for _, c := range chips {
		if c.ready {
			n++
		}
	}
	return n, len(chips)
}

// docsLabel is the list's one-slot document state: "✓ ready" once the Application can be
// sent, otherwise how many documents are ready, "○ 2/4 ready", amber once the Resume has
// text and only the build is wanting.
func (p pal) DocsLabel(a store.Application) (string, color.Color) {
	n, total := readyCount(a)
	switch a.Documents {
	case store.DocsReady:
		return glyphReady + " " + docReady, p.Ok
	case store.DocsWritten:
		return fmt.Sprintf("%s %d/%d %s", glyphMissing, n, total, docReady), p.Warn
	}
	return fmt.Sprintf("%s %d/%d %s", glyphMissing, n, total, docReady), p.Dim
}

// meterGlyphs are the ready meter's filled and empty cells.
const (
	meterOn  = glyphMeterOn
	meterOff = glyphMeterOff
)

// meterCells is the meter's cells, one per needed document: ■■□□ for two ready of four.
// st styles each run, so a selected row keeps its fill under the cells.
func (p pal) meterCells(a store.Application, st func(color.Color) lipgloss.Style) string {
	n, total := readyCount(a)
	c := p.Dim
	if n == total {
		c = p.Ok
	}
	return st(c).Render(strings.Repeat(meterOn, n)) + st(p.Faint).Render(strings.Repeat(meterOff, total-n))
}

// DocsMeter is the detail's "■■■□ 3/4 ready": the cells, then how many documents are ready.
func (p pal) DocsMeter(a store.Application) string {
	n, total := readyCount(a)
	c := p.Dim
	if n == total {
		c = p.Ok
	}
	return p.meterCells(a, p.Text) + p.Text(c).Render(fmt.Sprintf(" %d/%d %s", n, total, docReady))
}

// readyTotals is the documents ready of the documents needed across the Workspace's
// active Applications: those still live (not Dismissed, Rejected or Withdrawn, and
// readable). It is the share the pipeline strip shows.
func readyTotals(apps []store.Application) (ready, needed int) {
	for _, a := range apps {
		if !isActive(a) {
			continue
		}
		n, total := readyCount(a)
		ready += n
		needed += total
	}
	return ready, needed
}

// readyPercent is readyTotals as a whole percent, 0 when nothing is needed.
func readyPercent(apps []store.Application) int {
	ready, needed := readyTotals(apps)
	if needed == 0 {
		return 0
	}
	return (ready*100 + needed/2) / needed
}

// isActive reports whether an Application is still live: it reads, and is not closed or dismissed.
func isActive(a store.Application) bool {
	t := tableOf(a)
	return a.Malformed == nil && t != tableDismissed && t != tableClosed
}
