package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// The row's posting-age words say what the schedule rules decide for every shared
// posting_age vector (fixtures/conformance.json, asserted in internal/schedule/tests):
// "past the age limit" once days_left is 0 or less, "expires in …" exactly when it warns.
func TestAgeWarningWordsTheScheduleRules(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "fixtures", "conformance.json"))
	if err != nil {
		t.Fatal(err)
	}
	var c struct {
		PostingAge []struct {
			Name              string
			Posted            string
			Today             string
			Source            string
			Sources           []store.Source
			MaxPostingAgeDays *int `json:"max_posting_age_days"`
			ExpiryWarningDays int  `json:"expiry_warning_days"`
		} `json:"posting_age"`
	}
	if err := json.Unmarshal(b, &c); err != nil {
		t.Fatal(err)
	}
	for _, v := range c.PostingAge {
		now, err := time.Parse(time.DateOnly, v.Today)
		if err != nil {
			t.Fatal(err)
		}
		now = now.Add(12 * time.Hour)
		rules := store.SearchRules{ExpiryWarningDays: v.ExpiryWarningDays}
		if v.MaxPostingAgeDays != nil {
			rules.MaxPostingAgeDays = *v.MaxPostingAgeDays
		}
		rules = rules.WithSources(v.Sources)
		a := store.Application{Status: store.StatusLead, Posted: v.Posted, Source: v.Source}
		warning := ageWarning(a, rules, now)
		left, ok := schedule.PostingDaysLeft(a, rules, now)
		if got, want := warning == "past the age limit", ok && left <= 0; got != want {
			t.Errorf("%s: %q, expired %v", v.Name, warning, want)
		}
		if got, want := strings.HasPrefix(warning, "expires"), schedule.PostingExpiring(a, rules, now); got != want {
			t.Errorf("%s: %q, warns %v", v.Name, warning, want)
		}
	}
}
