package ui

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The meter counts the needed documents that are ready: all four, none, and three of
// four with only the PDF missing, which is written but not Ready (ADR 0004).
func TestReadinessCountsNeededDocuments(t *testing.T) {
	texts := store.Application{JDHasText: true, ResumeHasText: true, CoverLetterHasText: true}
	allReady := texts
	allReady.Documents = store.DocsReady
	onlyPDFMissing := texts
	onlyPDFMissing.Documents = store.DocsWritten
	resumeOnly := store.Application{JDHasText: true, ResumeHasText: true, ResumeOnly: true, Documents: store.DocsReady}
	cases := []struct {
		name         string
		app          store.Application
		ready, total int
	}{
		{"all ready", allReady, 4, 4},
		{"none ready", store.Application{}, 0, 4},
		{"only the PDF missing", onlyPDFMissing, 3, 4},
		{"Resume only needs no letter", resumeOnly, 3, 3},
	}
	for _, c := range cases {
		if n, total := readyCount(c.app); n != c.ready || total != c.total {
			t.Errorf("%s: %d/%d, want %d/%d", c.name, n, total, c.ready, c.total)
		}
	}
	if onlyPDFMissing.Documents == store.DocsReady {
		t.Error("an Application with no PDF must not count as Ready")
	}
}

// The Workspace total is the share of needed documents that are ready over active
// Applications. Hand-computed from the fixture: ten active Applications need four
// documents and three (the status-* ones, Resume only) need three: 10x4 + 3x3 = 49.
// docs-partial has its Resume ready, docs-written its Resume and Cover letter, and the
// three lead-* ones their JD: 6 ready, 12%. Closed, Dismissed and unreadable Applications are left out.
func TestReadyTotalsOverActiveApplications(t *testing.T) {
	m := fixtureModel(t)
	ready, needed := readyTotals(m.apps)
	if ready != 6 || needed != 49 {
		t.Errorf("readyTotals = %d/%d, want 6/49", ready, needed)
	}
	if got := readyPercent(m.apps); got != 12 {
		t.Errorf("readyPercent = %d, want 12", got)
	}
	if got := readyPercent(nil); got != 0 {
		t.Errorf("readyPercent of nothing = %d, want 0", got)
	}
}
