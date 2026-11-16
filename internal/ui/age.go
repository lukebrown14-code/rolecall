package ui

import (
	"strconv"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// postedDays is how many whole days ago a Lead's ad was posted, and whether it says.
func postedDays(a store.Application, now time.Time) (int, bool) {
	_, d, ok := daysFrom(a.Posted, now)
	return -d, ok
}

// postedAge is a posting age as a row shows it, "12d", or "" when the ad has no date.
func postedAge(a store.Application, now time.Time) string {
	d, ok := postedDays(a, now)
	if !ok {
		return ""
	}
	return strconv.Itoa(max(d, 0)) + "d"
}

// ageWarning is what max_posting_age_days says of a Lead: "past the age limit" once its
// ad is older, "expires in N days" within expiry_warning_days of it, or "".
func ageWarning(a store.Application, rules store.SearchRules, now time.Time) string {
	if rules.MaxPostingAgeDays <= 0 || a.Status != store.StatusLead {
		return ""
	}
	d, ok := postedDays(a, now)
	if !ok {
		return ""
	}
	left := rules.MaxPostingAgeDays - d
	switch {
	case left < 0:
		return "past the age limit"
	case left == 0:
		return "expires today"
	case left == 1:
		return "expires in 1 day"
	case d >= rules.MaxPostingAgeDays-rules.ExpiryWarningDays:
		return "expires in " + strconv.Itoa(left) + " days"
	}
	return ""
}

// filterReasonLabel is a filter_reason as the Owner reads it.
func filterReasonLabel(reason string) string {
	switch reason {
	case store.FilterExpired:
		return "too old"
	case store.FilterBlocker:
		return "blocker"
	case store.FilterApplicantCap:
		return "too many applicants"
	case store.FilterLocation:
		return "outside Search area"
	case store.FilterContractLength:
		return "contract too long"
	case store.FilterClosed:
		return "closing date passed"
	case store.FilterPulled:
		return "listing pulled"
	case store.FilterDuplicate:
		return "Likely duplicate"
	}
	return reason
}
