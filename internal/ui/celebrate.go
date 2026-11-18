package ui

import (
	"fmt"
	"slices"
	"strings"

	"github.com/lukebrown14-code/rolecall/internal/schedule"
	"github.com/lukebrown14-code/rolecall/internal/store"
)

// offerPrefix starts the toast for a move to Offer, which the feedback line draws as a
// celebration: a gold ★ in place of the ✓.
const offerPrefix = "OFFER · "

// offerToast is the one-time banner when the Owner moves an Application to Offer, the
// moment the whole tool exists for: "OFFER · Orbit Labs ★  first offer, 41 days
// after applying, 12 sent": short enough that the breadcrumb keeps its place beside it. Like every toast it clears on the next key. Rejections get no such thing.
func (m model) offerToast(a store.Application) string {
	applied, offers := 0, 0
	for _, x := range m.apps {
		if x.AppliedDate != nil {
			applied++
		}
		if x.Folder != a.Folder && reachedOffer(x) {
			offers++
		}
	}
	name := strings.TrimSpace(a.Company)
	if name == "" {
		name = appName(a.Title)
	}
	var parts []string
	if offers == 0 {
		parts = append(parts, "first offer")
	}
	if a.AppliedDate != nil {
		if _, days, ok := schedule.DaysFrom(a.AppliedDate.At, m.now()); ok && days <= 0 {
			parts = append(parts, plural(-days, "day", "days")+" after applying")
		}
	}
	parts = append(parts, fmt.Sprintf("%d sent", applied))
	return offerPrefix + name + " ★  " + strings.Join(parts, ", ")
}

// reachedOffer reports whether an Application was ever at Offer.
func reachedOffer(a store.Application) bool {
	return slices.ContainsFunc(a.StatusLog, func(e store.StatusEntry) bool { return e.Status == store.StatusOffer })
}
