package ui

import (
	"fmt"
	"image/color"
	"strings"

	"charm.land/lipgloss/v2"

	"github.com/lukebrown14-code/rolecall/internal/store"
	"github.com/lukebrown14-code/rolecall/internal/ui/watch"
)

// An Application's files, each with one name everywhere (GLOSSARY). The Documents are the
// Resume and the Cover letter, each Written then Built; the JD and Gaps are inputs on
// their own tabs, never counted. The file a name lives in is shown, muted, only where the
// Owner may want to open it.
const (
	docJD          = "JD"
	docResume      = "Resume"
	docCoverLetter = "Cover letter"
	docGaps        = "Gaps"
	docPDF         = "PDF" // a Document's Built step, the PDF in out/
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

// The two steps of a Document: Written (its Markdown has text), then Built (its PDF is
// in out/).
const (
	stepWritten = "written"
	stepBuilt   = "built"
)

// docChip is one step of one Document in the set that makes an Application sendable.
type docChip struct {
	doc, step, file string
	ready           bool
}

// todo is what the step still wants, as the Docs-not-Ready prompt names it: "write the
// Resume", "build the Cover letter".
func (c docChip) todo() string {
	if c.step == stepBuilt {
		return "build the " + c.doc
	}
	return "write the " + c.doc
}

// docChips is the set that makes an Application sendable: the Resume and, unless it is
// Resume only, the Cover letter, each Written then Built (SCHEMA Ready). The JD and Gaps
// are inputs, shown on their own tabs and never counted.
func docChips(a store.Application) []docChip {
	chips := []docChip{
		{docResume, stepWritten, resumeFile, a.ResumeHasText},
		{docResume, stepBuilt, watch.PDFDocument, a.ResumeBuilt},
	}
	if !a.ResumeOnly {
		chips = append(chips,
			docChip{docCoverLetter, stepWritten, coverLetterFile, a.CoverLetterHasText},
			docChip{docCoverLetter, stepBuilt, watch.PDFDocument, a.CoverLetterBuilt})
	}
	return chips
}

// readyCount is how many of the Application's Document steps are done, of how many.
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

// docsLabel is the list's one-slot Document state: "✓ ready" once the Application can be
// sent, otherwise how many Document steps are done, "○ 2/4 ready" (Resume and Cover
// letter, each Written then Built), amber once the Resume has text and only the build is
// wanting.
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

// meterCells is the meter's cells, one per Document step: ■■□□ for two done of four.
// st styles each run, so a selected row keeps its fill under the cells.
func (p pal) meterCells(a store.Application, st func(color.Color) lipgloss.Style) string {
	n, total := readyCount(a)
	c := p.Dim
	if n == total {
		c = p.Ok
	}
	return st(c).Render(strings.Repeat(meterOn, n)) + st(p.Faint).Render(strings.Repeat(meterOff, total-n))
}

// DocsMeter is the detail's "■■■□ 3/4 ready": the cells, then how many Document steps are done.
func (p pal) DocsMeter(a store.Application) string {
	n, total := readyCount(a)
	c := p.Dim
	if n == total {
		c = p.Ok
	}
	return p.meterCells(a, p.Text) + p.Text(c).Render(fmt.Sprintf(" %d/%d %s", n, total, docReady))
}

// readyTotals is the Document steps done of the steps needed across the Workspace's
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
	return a.Malformed == nil && t != tableDismissed && t != tableFiltered && t != tableClosed
}
