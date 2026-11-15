package ui

import (
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/lukebrown14-code/rolecall/internal/reporting"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The Stats report is built once per interaction: the scroll bound a key asks for and the
// frame drawn from the same model share one build, instead of each rescanning the
// Applications.
func TestStatsReportBuildsOncePerUpdate(t *testing.T) {
	m := fixtureModel(t)
	m.screen, m.width, m.height = 3, 60, 16

	builds := 0
	real := buildStatsReport
	buildStatsReport = func(apps []store.Application, now time.Time, followUpDays ...int) reporting.StatsReport {
		builds++
		return real(apps, now, followUpDays...)
	}
	t.Cleanup(func() { buildStatsReport = real })

	// One interaction: a scroll key asks for the scroll bound, and the frame then asks
	// for the same report; both reuse the one build.
	next, _ := m.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	if builds != 1 {
		t.Fatalf("Update built the Stats report %d times, want 1", builds)
	}
	_ = next.View()
	if builds != 1 {
		t.Fatalf("view built the Stats report again: %d builds, want 1", builds)
	}

	// The next interaction builds once more, not once per sub-view.
	next, _ = next.Update(tea.KeyPressMsg{Code: 'j', Text: "j"})
	_ = next.View()
	if builds != 2 {
		t.Fatalf("two interactions made %d builds, want 2", builds)
	}
}

// A funnel rung's note reads true beside its bar: a share of the previous rung only when
// the whole rung came through it, and counts when some did not, never "0% of".
func TestFunnelNoteReadsTrue(t *testing.T) {
	for _, c := range []struct {
		step reporting.FunnelStep
		prev string
		want string
	}{
		{reporting.FunnelStep{Count: 4, Through: 4, Conversion: 36, ConversionOK: true}, store.StatusApplied, "36% of Applied"},
		{reporting.FunnelStep{Count: 0, ConversionOK: true}, store.StatusApplied, "0% of Applied"},
		{reporting.FunnelStep{Count: 2, Skipped: 2, ConversionOK: true}, store.StatusLead, "none came from Leads"},
		{reporting.FunnelStep{Count: 11, Through: 3, Skipped: 8, Conversion: 50, ConversionOK: true}, store.StatusDraft, "3 came from Drafts · 8 didn't"},
		{reporting.FunnelStep{Count: 2, Skipped: 2}, store.StatusLead, "none came from Leads"},
		{reporting.FunnelStep{}, store.StatusLead, ""},
	} {
		if got := funnelNote(c.step, c.prev); got != c.want {
			t.Errorf("funnelNote(%+v, %s) = %q, want %q", c.step, c.prev, got, c.want)
		}
	}
}
