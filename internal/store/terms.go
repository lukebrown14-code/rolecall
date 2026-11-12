package store

// Statuses are the values of a status_log entry (SCHEMA.md), in pipeline order.
const (
	StatusLead         = "Lead"
	StatusDraft        = "Draft"
	StatusApplied      = "Applied"
	StatusInterviewing = "Interviewing"
	StatusOffer        = "Offer"
	StatusRejected     = "Rejected"
	StatusWithdrawn    = "Withdrawn"
	StatusDismissed    = "Dismissed"
)

// Statuses lists every Status in pipeline order.
var Statuses = []string{StatusLead, StatusDraft, StatusApplied, StatusInterviewing, StatusOffer, StatusRejected, StatusWithdrawn, StatusDismissed}

// Documents states (SCHEMA.md → Documents state); "" means nothing written yet.
const (
	DocsWritten = "Written"
	DocsReady   = "Ready"
)

// Fit values, strongest first; "" means not assessed.
const (
	FitStrong   = "Strong"
	FitGood     = "Good"
	FitMarginal = "Marginal"
	FitPoor     = "Poor"
)

// Required meta.yml keys (SCHEMA.md).
const (
	keyTitle   = "title"
	keyCompany = "company"
	keyTrack   = "track"
)
