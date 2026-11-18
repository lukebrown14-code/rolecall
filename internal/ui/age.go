package ui

import (
	"strconv"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// postedAge is a posting age as a row shows it, "12d", or "" when the ad has no date.
func postedAge(a store.Application, now time.Time) string {
	d, ok := schedule.PostedDays(a, now)
	if !ok {
		return ""
	}
	return strconv.Itoa(max(d, 0)) + "d"
}

// ageWarning is what max_posting_age_days says of a Lead: "past the age limit" once its
// ad reaches it, "expires in N days" within expiry_warning_days of it ("expires in 1 day"
// on its last day), or "".
func ageWarning(a store.Application, rules store.SearchRules, now time.Time) string {
	left, ok := schedule.PostingDaysLeft(a, rules, now)
	switch {
	case !ok:
		return ""
	case left <= 0:
		return "past the age limit"
	case left > rules.ExpiryWarningDays:
		return ""
	case left == 1:
		return "expires in 1 day"
	}
	return "expires in " + strconv.Itoa(left) + " days"
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
