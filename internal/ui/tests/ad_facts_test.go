package ui_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The detail shows what the ad says: the recruiter, its terms, pay with the yearly
// estimate, sponsorship, how it was found.
func TestDetailShowsAdFacts(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 140, 50)
	d.Type("1")
	d.Type("/Quokka")
	d.Type("enter")
	d.Type("enter")
	requireAll(t, squash(d.plain()), "Quokka Energy via Wattle Talent", "RECRUITER Wattle Talent · for Quokka Energy",
		"TYPE contract · 6 months · extension possible", "PAY $900–1,100/day AUD · ~A$207k–253k/yr", "SPONSORSHIP offered",
		`ADDED BY sweep · "data engineer melbourne"`)
}

// A pay figure whose unit looks wrong is flagged under the estimate.
func TestDetailFlagsAMislabelledRate(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 140, 50)
	d.Type("1")
	d.Type("/Bilby")
	d.Type("enter")
	d.Type("enter")
	requireAll(t, squash(d.plain()), "PAY $850/hour AUD · ~A$1680k/yr", "⚠ looks like a day rate", "CLOSES date unclear")
}

// Lead rows carry the visa and ad badges after the dates, worded for the role, blockers
// first and marked ⚠ so they read without colour.
func TestLeadRowsCarryVisaBadges(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 200, 40)
	d.Type("1")
	v := d.plain()
	requireAll(t, v, "contract fits visa cap · sponsors visa", "ad summary only",
		"closes: date unclear · ⚠ 12-mo contract > 6-mo visa cap · ⚠ no sponsorship")
	for _, old := range []string{"blurb only", "needs sponsorship", "month cap"} {
		if strings.Contains(v, old) {
			t.Fatalf("old badge wording %q:\n%s", old, v)
		}
	}
}

// A blocker badge leads the badges, so a row cut short drops the lighter ones first.
func TestBlockerBadgesComeFirst(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, func(root string) {
		meta := filepath.Join(root, "jobs", "SWE", "lead-blurb", "meta.yml")
		b, err := os.ReadFile(meta)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(meta, []byte(string(b)+"sponsorship: not-offered\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}), fixtureClock, 200, 40)
	d.Type("1")
	requireAll(t, lineWith(d.plain(), "Numbat Health"), "SWE · ⚠ no sponsorship · ad summary only")
}

// Deadlines, on the Drafts screen, lists every Lead and Draft with a deadline still to
// come, a Lead whose ad nears max_posting_age_days included, then the unclear ones.
func TestDeadlinesListsLeadsAndDraftsByCloseDate(t *testing.T) {
	d := newDriver(t, fixtureCopy(t, nil), fixtureClock, 140, 40)
	d.Type("2")
	d.Type("s")
	v := d.plain()
	requireAll(t, v, "[Deadlines 2]", "Deadlines · 1", "Platform Engineer", "expires in 3 days", "Date unclear · 1", "Reliability Engineer")
	if strings.Contains(v, "Data Engineer") {
		t.Fatalf("a Lead with no deadline is listed:\n%s", v)
	}
}

// Add warns of a Likely duplicate and saves on a second enter.
func TestAddWarnsOfALikelyDuplicate(t *testing.T) {
	h := newAddHarness(t)
	h.fillAdd("Data Engineer", "Quokka Energy", "", "")
	requireAll(t, h.tm.plain(), "Likely duplicate of", "lead-contract")
	h.enter()
	h.wait("Added Lead")
}

// Settings › Rules toggles Needs sponsorship and shows the visa cap.
func TestSettingsVisaRows(t *testing.T) {
	h := newSettingsHarness(t, 120, `{"visa":{"max_contract_months":6}}`, nil)
	h.tm.Type("6")
	h.tm.Type("]")
	requireAll(t, squash(h.tm.plain()), "Needs sponsorship No", "Contract cap (visa) 6 months workspace.json")
	for range 5 {
		h.tm.Type("j")
	}
	h.tm.Type("enter")
	requireAll(t, h.tm.plain(), "Saved Needs sponsorship: Yes")
	if f := h.file(); !strings.Contains(f, `"needs_sponsorship": true`) {
		t.Fatalf("workspace.json:\n%s", f)
	}
	h.tm.Type("u")
	if f := h.file(); strings.Contains(f, "needs_sponsorship") {
		t.Fatalf("undo left it:\n%s", f)
	}
}
