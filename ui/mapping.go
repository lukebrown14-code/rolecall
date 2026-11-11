package ui

// StatusLocation maps an Application Status to its screen and table.
func StatusLocation(status string) (screen, table string) {
	switch status {
	case "Lead":
		return "Leads", "Leads"
	case "Dismissed":
		return "Leads", "Dismissed"
	case "Draft":
		return "Draft", "Draft"
	case "Applied", "Interviewing", "Offer":
		return "Applied", status
	case "Rejected", "Withdrawn":
		return "Applied", "Closed"
	default:
		return "Leads", "Leads"
	}
}
