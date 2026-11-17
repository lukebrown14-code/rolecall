package ui

import (
	"sort"
	"time"

	"github.com/lukebrown14-code/rolecall/internal/store"
)

// SoonDays is how far ahead "now" looks: a close date or a next step within it is wanted
// today. The rows' amber close date turns at the same distance.
const SoonDays = soonDays

// Now is what wants doing today: the Follow-up due Applications, the next steps coming
// up, and the Leads and Drafts closing soon. Stats → Now draws it and `rolecall now`
// prints it, so the two can never count differently.
type Now struct {
	FollowUps []store.Application
	NextSteps []store.Application // soonest first
	Closing   []store.Application // soonest first
}

// Empty reports that nothing is due.
func (n Now) Empty() bool { return len(n.FollowUps)+len(n.NextSteps)+len(n.Closing) == 0 }

// openStatus is a Status whose next step still matters: not closed, not dismissed.
func openStatus(status string) bool {
	return status != store.StatusRejected && status != store.StatusWithdrawn && status != store.StatusDismissed
}

// NowFor works out Now from the Applications as of now. followUpDays is the follow-up
// period; within is how many days ahead a next step or close date counts (SoonDays).
// A malformed Application is left out.
func NowFor(apps []store.Application, now time.Time, followUpDays, within int) Now {
	var n Now
	for _, a := range apps {
		if a.Malformed != nil {
			continue
		}
		if followUpDue(a, now, followUpDays) {
			n.FollowUps = append(n.FollowUps, a)
		}
		if a.NextStep != nil && openStatus(a.Status) {
			if _, days, ok := daysFrom(a.NextStep.Day(), now); ok && days >= 0 && days <= within {
				n.NextSteps = append(n.NextSteps, a)
			}
		}
		if a.Unsent() {
			if _, days, ok := daysFrom(a.Closes, now); ok && days >= 0 && days <= within {
				n.Closing = append(n.Closing, a)
			}
		}
	}
	sort.SliceStable(n.NextSteps, func(i, j int) bool { return n.NextSteps[i].NextStep.At < n.NextSteps[j].NextStep.At })
	sort.SliceStable(n.Closing, func(i, j int) bool { return n.Closing[i].Closes < n.Closing[j].Closes })
	return n
}
