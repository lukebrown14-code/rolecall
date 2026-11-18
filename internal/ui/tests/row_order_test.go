package ui_test

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/lukebrown14-code/rolecall/internal/config"
	"github.com/lukebrown14-code/rolecall/internal/ui"
)

// writeLead writes a Lead with the meta.yml lines given (closes, posted, fit), added on
// the fixture day.
func writeLead(t *testing.T, root, name string, lines ...string) {
	t.Helper()
	meta := "schema: 1\ntitle: " + name + "\ncompany: Co " + name + "\ntrack: SWE\n" + strings.Join(lines, "\n") +
		"\nstatus_log:\n  - {status: Lead, at: 2026-09-30}\n"
	writeFile(t, filepath.Join(root, "jobs", "SWE", strings.ToLower(name), "meta.yml"), meta)
}

// leadOrder is the titles of the Leads view's rows, top to bottom, among names.
func leadOrder(view string, names ...string) []string {
	var out []string
	for _, l := range strings.Split(view, "\n") {
		row, _, _ := strings.Cut(l, "│")
		for _, n := range names {
			if strings.Contains(row, "◌ "+n+" ") || strings.HasSuffix(strings.TrimRight(row, " "), "◌ "+n) {
				out = append(out, n)
			}
		}
	}
	return out
}

// Leads sort by deadline: the close date or the day the ad passes max_posting_age_days
// (30 in the fixture), whichever is sooner, so a Lead about to age out sorts with the
// dated ones. Leads with no deadline follow, best Fit first, then newest posted.
func TestLeadsSortByDeadlineThenFitThenPosted(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		writeLead(t, root, "Aging", "posted: 2026-09-02") // 29 days old: expired from tomorrow, 2 Oct
		writeLead(t, root, "GoodOld", "fit: Good", "closes: 2026-10-10", "posted: 2026-09-20")
		writeLead(t, root, "GoodNew", "fit: Good", "closes: 2026-10-10", "posted: 2026-09-28")
		writeLead(t, root, "Dated", "closes: 2026-10-20")
		writeLead(t, root, "Marginal", "fit: Marginal") // no deadline
		writeLead(t, root, "Strong", "fit: Strong")     // no deadline
	})
	v := chipped(runKeysSized(t, root, 120, 60, "1"))
	names := []string{"Aging", "GoodNew", "GoodOld", "Dated", "Strong", "Marginal"}
	if got := leadOrder(v, names...); strings.Join(got, " ") != strings.Join(names, " ") {
		t.Fatalf("Leads order = %v, want %v:\n%s", got, names, v)
	}
	if !strings.Contains(v, "expires in 1 day") {
		t.Fatalf("the aging Lead does not say it expires:\n%s", v)
	}
}

// A Lead past max_posting_age_days is Expired, like one past its close date: it sorts
// under the Expired section, whose ad is too old to send.
func TestLeadPastTheAgeLimitIsExpired(t *testing.T) {
	root := fixtureCopy(t, func(root string) { writeLead(t, root, "Stale", "posted: 2026-08-01") })
	v := chipped(runKeysSized(t, root, 120, 60, "1"))
	expired := strings.Index(v, "Expired · 1")
	if expired < 0 || strings.Index(v, "Stale") < expired || !strings.Contains(v, "past the age limit") {
		t.Fatalf("a Lead past the age limit is not under Expired:\n%s", v)
	}
}

// An ad exactly max_posting_age_days old is expired, as rules.py check reads it, and a
// Lead from a Source with its own rules.max_posting_age_days is held to that limit, matched
// by name ignoring case. Another Source's Lead keeps the global 30.
func TestPostingAgeLimitIsInclusiveAndPerSource(t *testing.T) {
	root := fixtureCopy(t, func(root string) {
		path := filepath.Join(root, "workspace.json")
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		ws := strings.Replace(string(b), `"name":"linkedin",`, `"name":"linkedin","rules":{"max_posting_age_days":20},`, 1)
		writeFile(t, path, ws)
		writeLead(t, root, "AtLimit", "posted: 2026-09-01")                        // 30 days, global 30
		writeLead(t, root, "SourceAt", "posted: 2026-09-11", "source: LinkedIn")   // 20 days, LinkedIn 20
		writeLead(t, root, "SourceNear", "posted: 2026-09-12", "source: linkedin") // 19 days
		writeLead(t, root, "OtherSource", "posted: 2026-09-11", "source: greenhouse")
	})
	v := chipped(runKeysSized(t, root, 160, 60, "1"))
	expired := strings.Index(v, "Expired · 2")
	if expired < 0 {
		t.Fatalf("want AtLimit and SourceAt under Expired · 2:\n%s", v)
	}
	for _, n := range []string{"AtLimit", "SourceAt"} {
		if strings.Index(v, "◌ "+n) < expired {
			t.Errorf("%s is not under Expired:\n%s", n, v)
		}
	}
	for _, n := range []string{"SourceNear", "OtherSource"} {
		if i := strings.Index(v, "◌ "+n); i < 0 || i > expired {
			t.Errorf("%s should be live, above Expired:\n%s", n, v)
		}
	}
	// row is a Lead's row and the row words under it, up to the next row.
	row := func(name string) string {
		_, after, _ := strings.Cut(v, "◌ "+name)
		r, _, _ := strings.Cut(after, "◌ ")
		return r
	}
	if r := row("SourceNear"); !strings.Contains(r, "expires in 1 day") {
		t.Errorf("SourceNear row = %q, want it to expire in 1 day", r)
	}
	if r := row("OtherSource"); strings.Contains(r, "expires") || strings.Contains(r, "past the age limit") {
		t.Errorf("OtherSource row = %q, want no age warning under the global 30", r)
	}
}

// In the Applied and Interviewing sections the Follow-ups due come first, most overdue
// first, so the first row under "Applied · N · K follow-up due" is the one to chase.
func TestFollowUpsDueSortFirstMostOverdueFirst(t *testing.T) {
	v := plain(driveModel(t, driveStart(t, heatWorkspace(t), fixtureClock, 120, 40), "3", "s"))
	var order []string
	for _, l := range strings.Split(v, "\n") {
		row, _, _ := strings.Cut(l, "│")
		for _, n := range []string{"heat-high", "heat-mid", "heat-low", "heat-none", "Software Engineer"} {
			if strings.Contains(row, "● "+n) {
				order = append(order, n)
			}
		}
	}
	want := "heat-high heat-mid heat-low heat-none Software Engineer"
	if got := strings.Join(order, " "); got != want {
		t.Fatalf("Applied order = %q, want %q:\n%s", got, want, v)
	}
}

// Under NO_COLOR a row's meaning still reads: a close date within a week is bold, a
// blocker badge leads with ⚠ (! in ASCII), and no Fit, no badge and no close date is
// told apart by colour alone.
func TestRowMeaningSurvivesNoColor(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	root := fixtureCopy(t, func(root string) {
		writeLead(t, root, "Soon", "closes: 2026-10-05")
		writeLead(t, root, "Later", "closes: 2026-10-20")
	})
	raw := driveModel(t, driveStart(t, root, fixtureClock, 200, 50), "1").View().Content
	boldBefore := func(word string) bool {
		for _, l := range strings.Split(raw, "\n") {
			if i := strings.Index(l, word); i >= 0 {
				return strings.HasPrefix(l[max(0, strings.LastIndex(l[:i], "\x1b[")):i], "\x1b[1")
			}
		}
		t.Fatalf("%q not drawn", word)
		return false
	}
	if !boldBefore("closes in 4d") || boldBefore("closes in 2w") {
		t.Fatalf("a close date within a week is not set apart by weight")
	}
	v := chipped(raw)
	for _, badge := range []string{"no sponsorship", "12-mo contract > 6-mo visa cap"} {
		if !strings.Contains(v, "⚠ "+badge) {
			t.Fatalf("blocker %q lacks ⚠:\n%s", badge, v)
		}
	}
	t.Setenv("ROLECALL_ASCII", "1")
	if v := chipped(driveModel(t, driveStart(t, root, fixtureClock, 200, 50), "1").View().Content); !strings.Contains(v, "! no sponsorship") {
		t.Fatalf("ASCII blocker lacks !:\n%s", v)
	}
}

// manyLeads is the fixture with n more Leads, each with a close date, company and Fit.
func manyLeads(t *testing.T, n int) string {
	return fixtureCopy(t, func(root string) {
		for i := range n {
			writeLead(t, root, fmt.Sprintf("Role%02d", i), fmt.Sprintf("closes: 2026-10-%02d", 2+i%27), "fit: Good")
		}
	})
}

// z switches compact rows at wide sizes: one line per row, title, company and track in
// aligned columns, so a 166×44 terminal shows at least 32 rows. It is saved to
// config.toml, and listed in ? help.
func TestCompactRowsShowTwiceAsMany(t *testing.T) {
	home := t.TempDir()
	d := newDriver(t, manyLeads(t, 40), fixtureClock, 166, 44, ui.WithConfigHome(home))
	d.Type("1")
	before := len(leadOrder(d.plain(), leadNames(40)...))
	d.Type("z")
	v := d.plain()
	after := leadOrder(v, leadNames(40)...)
	if before >= 32 || len(after) < 32 {
		t.Fatalf("compact rows show %d (two-line rows %d), want at least 32:\n%s", len(after), before, v)
	}
	company := regexp.MustCompile(`Co Role\d\d`)
	col := -1
	for _, l := range strings.Split(v, "\n") {
		row, _, _ := strings.Cut(l, "│")
		if loc := company.FindStringIndex(row); loc != nil {
			if at := ansi.StringWidth(row[:loc[0]]); col < 0 {
				col = at
			} else if at != col {
				t.Fatalf("company column starts at %d, want %d: %q", at, col, row)
			}
		}
	}
	if col < 0 {
		t.Fatalf("no company column in compact rows:\n%s", v)
	}
	b, err := os.ReadFile(config.Path(home))
	if err != nil || !strings.Contains(string(b), "compact_rows = true") {
		t.Fatalf("compact_rows not saved: %q %v", b, err)
	}
	d.Type("?")
	if !strings.Contains(helpText(d.View()), "z two-line rows") {
		t.Fatalf("help does not list z")
	}
	// Started with compact_rows, a narrow terminal keeps two-line rows: one line has no
	// room for every column there.
	narrow := newDriver(t, manyLeads(t, 40), fixtureClock, 80, 44, ui.WithCompactRows(true))
	narrow.Type("1")
	if n := len(leadOrder(narrow.plain(), leadNames(40)...)); n >= 32 {
		t.Fatalf("compact rows shown at 80 columns: %d rows", n)
	}
}

func leadNames(n int) []string {
	var out []string
	for i := range n {
		out = append(out, fmt.Sprintf("Role%02d", i))
	}
	return out
}
