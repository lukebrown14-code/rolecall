package ui

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The meter counts the Document steps done: the Resume and the Cover letter, each Written
// then Built. All four, none, and two of four with only the PDFs missing, which is
// Written but not Ready (ADR 0004). The JD and Gaps are inputs and never count.
func TestReadinessCountsNeededDocuments(t *testing.T) {
	texts := store.Application{JDHasText: true, GapsHasText: true, ResumeHasText: true, CoverLetterHasText: true}
	allReady := texts
	allReady.ResumeBuilt, allReady.CoverLetterBuilt, allReady.Documents = true, true, store.DocsReady
	onlyPDFsMissing := texts
	onlyPDFsMissing.Documents = store.DocsWritten
	resumeOnly := store.Application{JDHasText: true, ResumeHasText: true, ResumeBuilt: true, ResumeOnly: true, Documents: store.DocsReady}
	cases := []struct {
		name         string
		app          store.Application
		ready, total int
	}{
		{"all ready", allReady, 4, 4},
		{"none ready", store.Application{}, 0, 4},
		{"only the PDFs missing", onlyPDFsMissing, 2, 4},
		{"Resume only needs no letter", resumeOnly, 2, 2},
		{"the JD alone counts nothing", store.Application{JDHasText: true}, 0, 4},
	}
	for _, c := range cases {
		if n, total := readyCount(c.app); n != c.ready || total != c.total {
			t.Errorf("%s: %d/%d, want %d/%d", c.name, n, total, c.ready, c.total)
		}
	}
	if onlyPDFsMissing.Documents == store.DocsReady {
		t.Error("an Application with no PDF must not count as Ready")
	}
}

// The Workspace total is the share of Document steps done over active Applications.
// Hand-computed from the fixture: ten active Applications need four steps and three (the
// status-* ones, Resume only) need two: 10x4 + 3x2 = 46. docs-partial has its Resume
// written and built, docs-written its Resume and Cover letter written: 4 done, 9%.
// Closed, Dismissed and unreadable Applications are left out, and a JD counts nothing.
func TestReadyTotalsOverActiveApplications(t *testing.T) {
	m := fixtureModel(t)
	ready, needed := readyTotals(m.apps)
	if ready != 4 || needed != 46 {
		t.Errorf("readyTotals = %d/%d, want 4/46", ready, needed)
	}
	if got := readyPercent(m.apps); got != 9 {
		t.Errorf("readyPercent = %d, want 9", got)
	}
	if got := readyPercent(nil); got != 0 {
		t.Errorf("readyPercent of nothing = %d, want 0", got)
	}
}
