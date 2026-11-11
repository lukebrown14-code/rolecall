package ui

import "github.com/lukebrown14-code/rolecall/store"

// Screens, numbered 1-3.
const (
	screenLeads   = "Leads"
	screenDraft   = "Draft"
	screenApplied = "Applied"
)

// Tables: the views within a screen. Applied, Interviewing and Offer are named after
// their one Status.
const (
	tableLeads        = "Leads"
	tableDismissed    = "Dismissed"
	tableDraft        = "Draft"
	tableApplied      = store.StatusApplied
	tableInterviewing = store.StatusInterviewing
	tableOffer        = store.StatusOffer
	tableClosed       = "Closed"
)

// StatusLocation maps an Application Status to its screen and table.
func StatusLocation(status string) (screen, table string) {
	switch status {
	case store.StatusDismissed:
		return screenLeads, tableDismissed
	case store.StatusDraft:
		return screenDraft, tableDraft
	case store.StatusApplied, store.StatusInterviewing, store.StatusOffer:
		return screenApplied, status
	case store.StatusRejected, store.StatusWithdrawn:
		return screenApplied, tableClosed
	default: // Lead, and anything unreadable
		return screenLeads, tableLeads
	}
}
