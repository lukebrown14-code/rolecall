package schedule_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// knownDrift lists the shared conformance vectors (fixtures/conformance.json) the derived
// scheduling rules do not meet yet, by the audit finding that tracks each
// (reports/code-audit-2026-11-18). A listed vector is skipped while it fails and fails once
// it passes, so whoever fixes the finding deletes its entry here.
// plugin/tests/test_conformance.py keeps the Python list; internal/store/tests the store's.
var knownDrift = map[string]struct {
	reason  string
	vectors []string
}{}

func driftFor(vector string) string {
	for id, d := range knownDrift {
		for _, v := range d.vectors {
			if v == vector {
				return id
			}
		}
	}
	return ""
}

// checkVector runs one shared vector as a subtest: problems are what it got wrong. A
// known-drift vector is skipped while wrong and fails once right, so its entry is removed.
func checkVector(t *testing.T, section, name string, problems func() []string) {
	t.Helper()
	vector := section + "/" + name
	t.Run(name, func(t *testing.T) {
		got := problems()
		id := driftFor(vector)
		switch {
		case id != "" && len(got) == 0:
			t.Errorf("%s now conforms: remove it from knownDrift[%s]", vector, id)
		case id != "":
			t.Skipf("known drift %s (%s): %s", id, knownDrift[id].reason, strings.Join(got, "; "))
		default:
			for _, p := range got {
				t.Error(p)
			}
		}
	})
}

type derivedConformance struct {
	Base     string
	FollowUp []struct {
		Name       string
		Now        string
		Days       int
		FollowedUp string `json:"followed_up"`
		NextStep   string `json:"next_step"`
		StatusLog  []struct {
			Status string
			At     string
			Approx bool
		} `json:"status_log"`
		Want struct {
			From     string
			Since    string
			Due      bool
			DaysPast *int `json:"days_past"`
		}
	} `json:"follow_up"`
	PostingAge []struct {
		Name              string
		Posted            string
		Today             string
		Source            string
		Sources           []store.Source
		MaxPostingAgeDays *int `json:"max_posting_age_days"`
		ExpiryWarningDays int  `json:"expiry_warning_days"`
		Want              struct {
			Expired  bool
			DaysLeft *int `json:"days_left"`
			Warn     bool
		}
	} `json:"posting_age"`
}

func readDerivedConformance(t *testing.T) derivedConformance {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "fixtures", "conformance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c derivedConformance
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestKnownDriftNamesSharedVectors(t *testing.T) {
	c := readDerivedConformance(t)
	names := map[string]bool{}
	for _, v := range c.FollowUp {
		names["follow_up/"+v.Name] = true
	}
	for _, v := range c.PostingAge {
		names["posting_age/"+v.Name] = true
	}
	for id, d := range knownDrift {
		for _, v := range d.vectors {
			if !names[v] {
				t.Errorf("knownDrift[%s] names %q, which is not a derived-rule vector in fixtures/conformance.json", id, v)
			}
		}
	}
}

func optInt(p *int) string {
	if p == nil {
		return "null"
	}
	return fmt.Sprint(*p)
}

// Follow-up due (the ⚑, NEXT, sorting, Stats → Now, `rolecall now`) reads the shared
// vectors as meta.py follow-up does. The clock is the vector's own, in its own zone, so
// the result never depends on the machine's TZ.
func TestFollowUpMatchesTheSharedConformanceFixture(t *testing.T) {
	c := readDerivedConformance(t)
	root := t.TempDir()
	if err := os.CopyFS(root, os.DirFS("../../../fixtures/workspace")); err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	for i, v := range c.FollowUp {
		checkVector(t, "follow_up", v.Name, func() []string {
			now, err := time.Parse(time.RFC3339, v.Now)
			if err != nil {
				return []string{err.Error()}
			}
			text := c.Base
			if v.FollowedUp != "" {
				text += "followed_up: " + v.FollowedUp + "\n"
			}
			if v.NextStep != "" {
				text += "next_step: {at: " + v.NextStep + "}\n"
			}
			text += "status_log:\n"
			for _, e := range v.StatusLog {
				approx := ""
				if e.Approx {
					approx = ", approx: true"
				}
				text += "  - {status: " + e.Status + ", at: " + e.At + approx + "}\n"
			}
			folder := filepath.Join(root, "jobs", "SWE", fmt.Sprintf("follow-up-%d", i))
			if err := os.MkdirAll(folder, 0o700); err != nil {
				return []string{err.Error()}
			}
			if err := os.WriteFile(filepath.Join(folder, "meta.yml"), []byte(text), 0o600); err != nil {
				return []string{err.Error()}
			}
			a, err := s.Rescan(folder)
			if err != nil || a == nil || a.Malformed != nil {
				return []string{fmt.Sprintf("rescan: %v %v", err, a)}
			}
			var problems []string
			from, ok := schedule.FollowUpFrom(*a, now.Location())
			if got := from.Format(time.DateOnly); !ok || got != v.Want.From {
				problems = append(problems, fmt.Sprintf("from %s (%v), want %s", got, ok, v.Want.From))
			}
			since := "last change"
			if schedule.FollowedUpLater(*a, now.Location()) {
				since = "followed_up"
			}
			if since != v.Want.Since {
				problems = append(problems, fmt.Sprintf("since %q, want %q", since, v.Want.Since))
			}
			past, due := schedule.FollowUpDaysPast(*a, now, v.Days)
			if due != v.Want.Due {
				problems = append(problems, fmt.Sprintf("due %v, want %v", due, v.Want.Due))
			}
			if schedule.FollowUpDue(*a, now, v.Days) != due {
				problems = append(problems, "FollowUpDue disagrees with FollowUpDaysPast")
			}
			var gotPast *int
			if schedule.FollowUpStatus(a.Status) && !schedule.NextStepBooked(*a, now) {
				gotPast = &past
			}
			if optInt(gotPast) != optInt(v.Want.DaysPast) {
				problems = append(problems, fmt.Sprintf("days_past %s, want %s", optInt(gotPast), optInt(v.Want.DaysPast)))
			}
			return problems
		})
	}
}

// A Lead's posting age reads the shared vectors as rules.py check does: SCHEMA's days_left
// (0 or less is expired), the row's "expires in N days" warning and the Deadlines day.
func TestPostingAgeMatchesTheSharedConformanceFixture(t *testing.T) {
	c := readDerivedConformance(t)
	for _, v := range c.PostingAge {
		checkVector(t, "posting_age", v.Name, func() []string {
			now, err := time.Parse(time.DateOnly, v.Today)
			if err != nil {
				return []string{err.Error()}
			}
			now = now.Add(12 * time.Hour)
			rules := store.SearchRules{ExpiryWarningDays: v.ExpiryWarningDays}
			if v.MaxPostingAgeDays != nil {
				rules.MaxPostingAgeDays = *v.MaxPostingAgeDays
			}
			rules = rules.WithSources(v.Sources)
			a := store.Application{Status: store.StatusLead, Posted: v.Posted, Source: v.Source}
			n, hasLeft := schedule.PostingDaysLeft(a, rules, now)
			expired := hasLeft && n <= 0
			var left *int
			if hasLeft && !expired {
				left = &n
			}
			var problems []string
			// The deadline (sorting, the Expired section) passes on the same day.
			if _, ok := schedule.PostingExpiry(a, rules); ok && schedule.Passed(a, rules, now) != expired {
				problems = append(problems, fmt.Sprintf("deadline passed %v, expired %v", !expired, expired))
			}
			if expired != v.Want.Expired {
				problems = append(problems, fmt.Sprintf("expired %v (days left %d), want %v", expired, n, v.Want.Expired))
			}
			if optInt(left) != optInt(v.Want.DaysLeft) {
				problems = append(problems, fmt.Sprintf("days_left %s, want %s", optInt(left), optInt(v.Want.DaysLeft)))
			}
			if warn := schedule.PostingExpiring(a, rules, now); warn != v.Want.Warn {
				problems = append(problems, fmt.Sprintf("warn %v, want %v", warn, v.Want.Warn))
			}
			return problems
		})
	}
}
