package ui_test

import (
	"testing"

	"github.com/lukebrown14-code/rolecall/internal/ui"
)

func TestStatusLocation(t *testing.T) {
	for _, c := range []struct{ s, screen, table string }{{"Lead", "Leads", "Leads"}, {"Dismissed", "Leads", "Dismissed"}, {"Draft", "Drafts", "Drafts"}, {"Applied", "Pipeline", "Applied"}, {"Interviewing", "Pipeline", "Interviewing"}, {"Offer", "Pipeline", "Offer"}, {"Rejected", "Pipeline", "Closed"}, {"Withdrawn", "Pipeline", "Closed"}} {
		s, tbl := ui.StatusLocation(c.s)
		if s != c.screen || tbl != c.table {
			t.Errorf("%s: %s/%s", c.s, s, tbl)
		}
	}
}
